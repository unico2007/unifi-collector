"""Phase-2 AI Insights: automatic analysis over the unifi_* metrics.

Cheap, explainable statistics on the CPU (no ML model, no GPU): z-score anomaly
detection over a short rolling window, plus Prometheus predict_linear forecasts.
The LLM only writes a short Azerbaijani synthesis at the end — all the numbers
come from Prometheus, so nothing is fabricated.
"""

import statistics
import time

from .clients import prom, llm
from .config import settings

# Severity ordering for sorting (lower = more urgent).
_RANK = {"error": 0, "warn": 1, "info": 2}

# Lazy TTL cache (see settings.insights_cache_seconds). The panel polls per
# browser and each compute costs an LLM call; this decouples LLM calls from the
# number of viewers/polls so small cloud free-tier quotas aren't burned.
_cache: dict | None = None
_cache_at: float = 0.0


def _range(hours: int = 3):
    end = int(time.time())
    return str(end - hours * 3600), str(end), "5m"


def _series(data: dict) -> list[tuple[dict, list[float]]]:
    out = []
    for r in data.get("data", {}).get("result", []):
        vals = []
        for _, v in r.get("values", []):
            try:
                vals.append(float(v))
            except (TypeError, ValueError):
                pass
        out.append((r.get("metric", {}), vals))
    return out


def _instant(data: dict) -> list[tuple[dict, float]]:
    out = []
    for r in data.get("data", {}).get("result", []):
        try:
            out.append((r.get("metric", {}), float(r.get("value", [0, "0"])[1])))
        except (TypeError, ValueError):
            pass
    return out


def _scalar(data: dict) -> float:
    """Single value from an aggregate query like count(...), else 0."""
    inst = _instant(data)
    return inst[0][1] if inst else 0.0


def _zscore(values: list[float]) -> tuple[float, float]:
    """Return (current_value, z_score_vs_history). z=0 if not enough data."""
    if len(values) < 6:
        return (values[-1] if values else 0.0), 0.0
    hist, cur = values[:-1], values[-1]
    mean = statistics.mean(hist)
    sd = statistics.pstdev(hist)
    if sd == 0:
        return cur, 0.0
    return cur, (cur - mean) / sd


async def _offline_devices() -> list[dict]:
    data = await prom.query('unifi_device_up == 0')
    out = []
    for labels, _ in _instant(data):
        name = labels.get("name") or labels.get("mac", "?")
        out.append({
            "level": "error",
            "title": f"{name} offline",
            "detail": f"{labels.get('type', 'cihaz')} hazırda əlçatan deyil (unifi_device_up=0).",
        })
    return out


async def _metric_anomalies(metric: str, unit: str, floor: float, z_thresh: float, what: str) -> list[dict]:
    start, end, step = _range()
    data = await prom.query_range(metric, start, end, step)
    out = []
    for labels, vals in _series(data):
        cur, z = _zscore(vals)
        if z >= z_thresh and cur >= floor:
            name = labels.get("name") or labels.get("mac", "?")
            out.append({
                "level": "warn",
                "title": f"{name}: {what} yüksəkdir",
                "detail": f"{what} indi {cur:.0f}{unit} — son saatların normasından kəskin yuxarı (z≈{z:.1f}).",
            })
    return out


async def _client_trend() -> list[dict]:
    start, end, step = _range()
    data = await prom.query_range('sum(unifi_clients_total)', start, end, step)
    series = _series(data)
    if not series:
        return []
    _, vals = series[0]
    cur, z = _zscore(vals)
    out = []
    if z <= -2.5 and cur >= 0:
        out.append({
            "level": "warn",
            "title": "Klient sayı kəskin azalıb",
            "detail": f"İndi {cur:.0f} klient — son saatların normasından aşağı (z≈{z:.1f}). Mümkün AP problemi.",
        })
    # Forecast 2h ahead (subquery syntax: range selector on a function result).
    fc = await prom.query('predict_linear(sum(unifi_clients_total)[1h:5m], 7200)')
    inst = _instant(fc)
    if inst:
        pred = inst[0][1]
        if abs(pred - cur) >= max(5, cur * 0.2):
            direction = "artacaq" if pred > cur else "azalacaq"
            out.append({
                "level": "info",
                "title": "Klient proqnozu (2 saat)",
                "detail": f"Bu templə klient sayı ~2 saata {pred:.0f}-ə çatacaq (indi {cur:.0f}, {direction}).",
            })
    return out


async def _memory_forecast() -> list[dict]:
    # Which devices will cross 90% memory within 4h at the current slope.
    fc = await prom.query('predict_linear(unifi_device_memory_percent[1h], 14400)')
    cur_data = await prom.query('unifi_device_memory_percent')
    cur_by_mac = {l.get("mac"): v for l, v in _instant(cur_data)}
    out = []
    for labels, pred in _instant(fc):
        cur = cur_by_mac.get(labels.get("mac"), 0.0)
        if pred >= 90 and pred > cur:
            name = labels.get("name") or labels.get("mac", "?")
            out.append({
                "level": "warn",
                "title": f"{name}: yaddaş dolur",
                "detail": f"Bu templə yaddaş ~4 saata {pred:.0f}%-ə çatacaq (indi {cur:.0f}%).",
            })
    return out


async def _reboot_devices() -> list[dict]:
    # Devices whose uptime is under 30 min likely rebooted recently.
    data = await prom.query(
        'unifi_device_uptime_seconds < 1800 and unifi_device_uptime_seconds > 60'
    )
    out = []
    for labels, up in _instant(data):
        name = labels.get("name") or labels.get("mac", "?")
        mins = int(up // 60)
        out.append({
            "level": "info",
            "title": f"{name} yenidən başladı",
            "detail": f"Cihaz təxminən {mins} dəqiqə əvvəl yenidən başladı (uptime {mins} dəq).",
        })
    return out


async def _weak_signal_clients() -> list[dict]:
    # Flag only when a real SHARE of Wi-Fi clients are weak, so it doesn't fire on
    # the handful of poor-signal clients that always exist.
    weak = _scalar(await prom.query('count(unifi_client_rssi < -75)'))
    total = _scalar(await prom.query('count(unifi_client_rssi < 0)'))
    out = []
    if total >= 20 and weak / total >= 0.30:
        pct = round(100 * weak / total)
        out.append({
            "level": "warn",
            "title": "Zəif WiFi siqnalı",
            "detail": f"Klientlərin {pct}%-də ({int(weak)}/{int(total)}) siqnal < -75 dBm — mümkün əhatə və ya AP problemi.",
        })
    return out


async def _sustained_high_memory() -> list[dict]:
    # Devices whose memory has been high for a while (complements the z-score
    # spike detector and the forecast: a flat-but-high device is caught here).
    data = await prom.query('avg_over_time(unifi_device_memory_percent[15m]) > 88')
    out = []
    for labels, avg in _instant(data):
        name = labels.get("name") or labels.get("mac", "?")
        out.append({
            "level": "warn",
            "title": f"{name}: yaddaş davamlı yüksək",
            "detail": f"Son 15 dəqiqədə yaddaş orta {avg:.0f}% — davamlı yüksəkdir (mümkün yaddaş sızması).",
        })
    return out


async def _safe(coro) -> list[dict]:
    """Run one insight collector, swallowing errors so a single bad query
    (e.g. an unsupported PromQL feature) never fails the whole endpoint."""
    try:
        return await coro
    except Exception:  # noqa: BLE001
        return []


async def compute() -> dict:
    """Gather all insights, rank them, and add a short LLM synthesis. Result is
    cached for settings.insights_cache_seconds so repeated polls (many browser
    tabs) don't each trigger an LLM call."""
    global _cache, _cache_at
    ttl = settings.insights_cache_seconds
    if _cache is not None and ttl > 0 and (time.time() - _cache_at) < ttl:
        return _cache

    insights: list[dict] = []
    insights += await _safe(_offline_devices())
    insights += await _safe(_metric_anomalies("unifi_device_cpu_percent", "%", 60, 2.5, "CPU"))
    insights += await _safe(_metric_anomalies("unifi_device_memory_percent", "%", 75, 2.5, "yaddaş"))
    insights += await _safe(_client_trend())
    insights += await _safe(_memory_forecast())
    insights += await _safe(_reboot_devices())
    insights += await _safe(_weak_signal_clients())
    insights += await _safe(_sustained_high_memory())

    insights.sort(key=lambda i: _RANK.get(i["level"], 3))

    if not insights:
        insights = [{
            "level": "info",
            "title": "Sistem normaldır",
            "detail": "Cari metriklərdə anomaliya və ya risk aşkarlanmadı.",
        }]

    summary = await _summarize(insights)
    result = {"insights": insights, "summary": summary, "generated_at": int(time.time())}
    _cache, _cache_at = result, time.time()
    return result


async def _summarize(insights: list[dict]) -> str:
    lines = "\n".join(f"- [{i['level']}] {i['title']}: {i['detail']}" for i in insights[:10])
    try:
        text = await llm.generate(
            f"Network monitoring findings (may be written in Azerbaijani):\n{lines}\n\n"
            "Summarize these in 1-2 sentences of clear English: what matters most and "
            "what to watch. Do not fabricate numbers; use only the given findings.",
            system="You are the Unico network monitoring assistant. Be short, concrete and practical, in English.",
        )
        return text.strip()
    except Exception:  # noqa: BLE001
        return ""
