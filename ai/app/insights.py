"""Phase-2 AI Insights: automatic analysis over the unifi_* metrics.

Cheap, explainable statistics on the CPU (no ML model, no GPU): z-score anomaly
detection over a short rolling window, plus Prometheus predict_linear forecasts.
The LLM only writes a short Azerbaijani synthesis at the end — all the numbers
come from Prometheus, so nothing is fabricated.
"""

import statistics
import time

from .clients import prom, llm

# Severity ordering for sorting (lower = more urgent).
_RANK = {"error": 0, "warn": 1, "info": 2}


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
    # Forecast 2h ahead.
    fc = await prom.query('predict_linear(sum(unifi_clients_total)[1h], 7200)')
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


async def compute() -> dict:
    """Gather all insights, rank them, and add a short LLM synthesis."""
    insights: list[dict] = []
    insights += await _offline_devices()
    insights += await _metric_anomalies("unifi_device_cpu_percent", "%", 60, 2.5, "CPU")
    insights += await _metric_anomalies("unifi_device_memory_percent", "%", 75, 2.5, "yaddaş")
    insights += await _client_trend()
    insights += await _memory_forecast()

    insights.sort(key=lambda i: _RANK.get(i["level"], 3))

    if not insights:
        insights = [{
            "level": "info",
            "title": "Sistem normaldır",
            "detail": "Cari metriklərdə anomaliya və ya risk aşkarlanmadı.",
        }]

    summary = await _summarize(insights)
    return {"insights": insights, "summary": summary, "generated_at": int(time.time())}


async def _summarize(insights: list[dict]) -> str:
    lines = "\n".join(f"- [{i['level']}] {i['title']}: {i['detail']}" for i in insights[:10])
    try:
        text = await llm.generate(
            f"Şəbəkə monitorinq nəticələri:\n{lines}\n\n"
            "Bunları 1-2 cümlədə Azərbaycanca ümumiləşdir: ən vacib nədir, nəyə diqqət etməli. "
            "Rəqəm uydurma, yalnız verilənləri istifadə et.",
            system="Sən şəbəkə monitorinq köməkçisisən. Qısa, konkret, praktiki danış.",
        )
        return text.strip()
    except Exception:  # noqa: BLE001
        return ""
