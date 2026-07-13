"""Phase-1 query agent: natural language -> PromQL/LogQL -> answer.

Single-step tool use: the LLM plans ONE query (Prometheus or Loki), the backend
executes it (validated), then the LLM answers from the result. Multi-step
reasoning, forecasting and anomaly tools are added in later phases behind the
same interface.
"""

from __future__ import annotations

import re

import httpx

from .clients import prom, loki, llm
from .config import settings
from .rag import kb

# Hand-written schema of the collector's neutral metrics — this grounds the LLM
# far better than raw metric names. Extend it as new vendors/metrics arrive.
SCHEMA = """
Prometheus metrics (namespace unifi_), labels in {}:
- unifi_device_up{vendor,site,mac,name,model,type}            1=online 0=offline (DEVICES: APs/switches/gateways)
- unifi_device_cpu_percent{...}                               device CPU %
- unifi_device_memory_percent{...}                            device memory %
- unifi_device_uptime_seconds{...}
- unifi_device_rx_bytes{...} / unifi_device_tx_bytes{...}     cumulative counters (use rate())
- unifi_device_info{vendor,site,mac,name,model,type,version,state,ip}  value=1; carries device ip/firmware/state
- unifi_devices_total{vendor,site,type}                       device count
- unifi_clients_total{vendor,site}                            connected client count
- unifi_client_rssi{vendor,site,mac,name,ap,vlan,band}        client signal dBm. A CLIENT (phone/laptop) has NO up/offline metric — it is CONNECTED iff it appears here.
- unifi_client_rx_rate / unifi_client_tx_rate{...}            radio LINK rate bits/s (negotiated speed, NOT real traffic)
- unifi_client_rx_bytes / unifi_client_tx_bytes{...}          bytes used THIS SESSION (real data volume)
- unifi_client_connected_seconds{...}                         how long the client has been connected
- unifi_client_info{vendor,site,mac,name,ip}                  value=1; carries the client's IP
- unifi_health_status{vendor,site,subsystem}
vendor is "unifi" or "kerio". Client "band" is "2.4 GHz" or "5 GHz".

Loki logs: labels {vendor,site,level,event}. Example stream selector {vendor="unifi"}.
Log levels: info, warn, error. Query with LogQL, e.g. {vendor="unifi"} |= "error".
""".strip()

PLANNER_SYSTEM = f"""You are a monitoring query planner+router for a UniFi + Kerio network.
Given a user question, decide how to answer it.
{SCHEMA}

Pick ONE source:
- "prometheus" — for live numbers/metrics/status (counts, CPU, memory, rssi, traffic).
- "loki" — for actual log text/events right now.
- "knowledge" — for how-to / why / "what does X mean" / network layout / procedures /
  documentation, when NO live data is needed. Put good search keywords in "query".
- "troubleshoot" — when the user reports a PROBLEM and wants to diagnose/fix it
  (something is broken / failing / not working / "why is X down"). This combines the
  runbook (knowledge) WITH live logs. Put runbook search keywords in "query" AND a
  targeted LogQL in "logql" (e.g. {{vendor="unifi"}} |= "keyword"); if unsure which
  logs, set "logql" to {{vendor=~"unifi|kerio"}}.

Rules:
- For rates/traffic use rate(metric[5m]).
- Keep any time range <= {'{'}max{'}'}.
- Output STRICT JSON: {{"source":"prometheus"|"loki"|"knowledge"|"troubleshoot","query":"...","logql":"...","reason":"..."}}
  ("logql" only matters for "troubleshoot"; use "" otherwise.)
- Do NOT invent metric or label names outside the schema.
- Add a label filter ONLY when the user explicitly names that value (a specific
  device name, client, AP, site, or vendor). For general questions ("the network",
  "all devices/clients", "how many ...") use the BARE metric with NO label
  selector — never guess a site/name/label the user did not say.
- NAME MATCHING: Prometheus matches label values EXACTLY and case-sensitively, but
  users almost never type the exact stored name. So when the user names a specific
  device/client/AP, match it with a case-insensitive SUBSTRING regex:
  name=~"(?i).*TERM.*"  — NOT  name="TERM".  (e.g. a phone the user calls "iphone"
  is stored as "iPhone" → name=~"(?i).*iphone.*".)
- Client names are the device HOSTNAME (e.g. "iPhone", "Galaxy-A50", "DESKTOP-…"),
  NOT a person's name. If the user asks about "<person>'s phone", still build the
  regex from the words they gave; if nothing matches, the answer will say so.
- For a CLIENT's "status": query unifi_client_rssi (presence = connected, value = signal).
- CLIENT vs DEVICE: clients are end-user gear (phones/laptops: "iPhone", "Galaxy-A50",
  "DESKTOP-…") → unifi_client_*. DEVICES are the network hardware (APs/switches/gateways,
  often named like "4.3.Vadim-AC-Lite", "…Nano HD", "…-AC-Lite", "…U7LT") → unifi_device_*.
  If the named thing looks like an AP/switch/gateway, use the device metrics. For
  "how much data/traffic through <device>" use
  rate(unifi_device_rx_bytes{{name=~"(?i).*TERM.*"}}[5m])+rate(unifi_device_tx_bytes{{name=~"(?i).*TERM.*"}}[5m]).
""".replace("{max}", "24h")

# Few-shot examples kept as a plain (non-f) string so the JSON braces don't need
# doubling. They anchor the "don't invent labels" rule — the exact failure where
# qwen turned "şəbəkədə" (in the network) into site="sebek" and got empty results.
PLANNER_EXAMPLES = (
    "\nExamples:\n"
    "Q: neçə cihaz offline?\n"
    '{"source":"prometheus","query":"count(unifi_device_up == 0)","reason":"offline count, general question so no label"}\n'
    "Q: şəbəkədə neçə klient var?\n"
    '{"source":"prometheus","query":"sum(unifi_clients_total)","reason":"total clients, no label filter"}\n'
    "Q: 5.2.Left_Nano HD cihazının CPU-su neçədir?\n"
    '{"source":"prometheus","query":"unifi_device_cpu_percent{name=~\\"(?i).*Left_Nano HD.*\\"}","reason":"named device, case-insensitive substring regex"}\n'
    "Q: Zeynalın iphone-u nə vəziyyətdədir?\n"
    '{"source":"prometheus","query":"unifi_client_rssi{name=~\\"(?i).*iphone.*\\"}","reason":"named client; rssi presence = connected; regex because stored name differs"}\n'
    "Q: Galaxy-A50 nə qədər data işlədib?\n"
    '{"source":"prometheus","query":"unifi_client_rx_bytes{name=~\\"(?i).*galaxy.*a50.*\\"}","reason":"session data volume for a named client, regex match"}\n'
)

PLANNER_SYSTEM = PLANNER_SYSTEM + PLANNER_EXAMPLES

# Answer language. Auto by default: Azerbaijani when ANY cloud model (Gemini or
# NVIDIA Qwen3) is active — both handle AZ well — else English, because the local
# 7B is weak at Azerbaijani (the reason answers were English before).
# AI_ANSWER_LANG overrides. Users ask in AZ either way; the planner stays English/JSON.
_LANG = settings.answer_lang or ("Azerbaijani" if (settings.gemini_api_key or settings.nvidia_api_key) else "English")

ANSWER_SYSTEM = f"""You are the Unico network monitoring assistant. Answer the user's
question in clear, concise {_LANG}, using ONLY the provided query result. Do not
fabricate numbers. If the result is empty, say so plainly and suggest what to check.
Keep it to 1-3 sentences; use short bullets only if genuinely helpful. Keep technical
terms as-is (CPU, RAM, IP, MAC, offline, online, AP, switch, gateway, VLAN, RSSI,
dBm, Mbps, uplink)."""

KNOWLEDGE_SYSTEM = f"""You are the Unico network monitoring assistant. Answer in clear,
concise {_LANG}, based ONLY on the "Knowledge" text below (it may itself be written in
Azerbaijani — read it and answer in {_LANG}). Do not invent anything not in it. If
there is no relevant information, say so and suggest what to check. Number the steps if
there are any. Cite the source at the end (e.g. source: runbooks.md)."""

TROUBLESHOOT_SYSTEM = f"""You are the Unico network monitoring assistant helping to fix a
problem. You get (a) runbook/knowledge text (may be in Azerbaijani) and (b) recent live
log lines from the network. Answer in clear, concise {_LANG}:
1. First, one line on what the live logs currently show (or say the logs show nothing
   relevant to this problem).
2. Then the concrete fix steps from the runbook, numbered.
Use ONLY the provided text — do not invent numbers, events or steps. Keep technical
terms as-is (CPU, IP, MAC, offline, AP, VLAN, RSSI, uplink). Cite the runbook source
at the end (e.g. source: runbooks.md)."""


def _validate(source: str, query: str) -> str | None:
    if source not in ("prometheus", "loki"):
        return "naməlum mənbə"
    if not query or len(query) > 2000:
        return "sorğu boş və ya çox uzundur"
    lowered = query.lower()
    for bad in ("delete", "drop", "admin_api", "/api/v1/admin"):
        if bad in lowered:
            return "sorğu qadağan olunan ifadə saxlayır"
    return None


def _metric_base(query: str) -> str | None:
    """Extract the bare metric name from a query like `unifi_client_rssi{name=~"..."}`."""
    m = re.match(r"\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\{", query)
    return m.group(1) if m else None


def _is_empty_prom(data: dict) -> bool:
    return not data.get("data", {}).get("result", [])


def _name_term(query: str) -> str:
    """Pull the searched name out of a name filter, e.g. name=~"(?i).*vadim.*" -> "vadim"."""
    m = re.search(r'name=~?"([^"]*)"', query)
    if not m:
        return ""
    raw = m.group(1)
    raw = re.sub(r"\(\?i\)|\.\*|\\", "", raw)  # strip regex noise
    return raw.strip().lower()


async def _prom_names(metric: str) -> list[str]:
    try:
        d = await prom.query(metric)
        return sorted({s.get("metric", {}).get("name", "") for s in d.get("data", {}).get("result", [])} - {""})
    except Exception:  # noqa: BLE001
        return []


async def _name_hint(query: str) -> str:
    """Build a helpful note for an empty named lookup: if the term is actually a
    device (or vice-versa), point there; otherwise list the names that exist."""
    term = _name_term(query)
    clients = await _prom_names("unifi_client_rssi")
    devices = await _prom_names("unifi_device_up")
    looked_client = "unifi_client" in query
    # The pool the user probably meant is the OTHER one if the term matches there.
    dev_hits = [n for n in devices if term and term in n.lower()]
    cli_hits = [n for n in clients if term and term in n.lower()]
    if looked_client and dev_hits:
        return ("\n\n(Bu ad KLİENT deyil — CİHAZ(lar)a uyğun gəlir: "
                + ", ".join(dev_hits[:10]) + ". Cihaz haqqında soruşun, məs. onun CPU-su/trafiki/statusu.)")
    if not looked_client and cli_hits:
        return ("\n\n(Bu ad CİHAZ deyil — KLİENT(lər)ə uyğun gəlir: "
                + ", ".join(cli_hits[:10]) + ".)")
    pool = clients if looked_client else devices
    if pool:
        label = "klient" if looked_client else "cihaz"
        return (f"\n\n(Bu ada uyğun {label} yoxdur. Mövcud {label} adları: " + ", ".join(pool[:25]) + ")")
    return ""


def _summarize(source: str, data: dict, limit: int = 40) -> str:
    """Compact the raw Prom/Loki JSON into a small text the LLM can read."""
    try:
        if source == "prometheus":
            res = data.get("data", {}).get("result", [])
            lines = []
            for s in res[:limit]:
                metric = s.get("metric", {})
                val = s.get("value", ["", ""])[1] if "value" in s else (s.get("values", [["", ""]])[-1][1])
                lines.append(f"{metric} = {val}")
            return "\n".join(lines) or "(nəticə boşdur)"
        else:
            streams = data.get("data", {}).get("result", [])
            lines = []
            for st in streams:
                labels = st.get("stream", {})
                for _, line in st.get("values", [])[:limit]:
                    lines.append(f"[{labels.get('level','')}] {line}")
            return "\n".join(lines[:limit]) or "(log tapılmadı)"
    except Exception as e:  # noqa: BLE001
        return f"(nəticəni oxumaq mümkün olmadı: {e})"


async def _answer_knowledge(question: str, refined: str) -> dict:
    """Phase-3 RAG branch: retrieve relevant knowledge chunks and answer from them."""
    hits = [h for h in await kb.search(refined or question) if h.score >= settings.rag_min_score]
    if not hits:
        note = f" ({kb.error})" if kb.error else ""
        return {
            "answer": "Bu barədə bilik bazasında uyğun məlumat tapmadım" + note
            + ". Sualı bir az dəqiqləşdirin, ya da metrik/log şəklində soruşun "
            "(məs. 'neçə cihaz offline?').",
            "source": "knowledge", "query": refined or question, "result": "",
        }
    context = "\n\n".join(f"[{h.chunk.source} › {h.chunk.section}]\n{h.chunk.text}" for h in hits)
    answer = await llm.generate(f"Sual: {question}\n\nBilik:\n{context}", system=KNOWLEDGE_SYSTEM)
    sources = sorted({h.chunk.source for h in hits})
    return {
        "answer": answer.strip(), "source": "knowledge",
        "query": refined or question, "result": context, "sources": sources,
    }


async def _answer_troubleshoot(question: str, keywords: str, logql: str) -> dict:
    """Hybrid branch: combine RAG runbook steps with live log evidence so the answer
    is grounded in BOTH the documented procedure and the current state."""
    hits = [h for h in await kb.search(keywords or question) if h.score >= settings.rag_min_score]
    knowledge = "\n\n".join(
        f"[{h.chunk.source} › {h.chunk.section}]\n{h.chunk.text}" for h in hits
    ) or "(uyğun runbook tapılmadı)"

    # Use the planner's LogQL if it's valid, else a safe recent-logs default.
    lq = logql if logql and _validate("loki", logql) is None else '{vendor=~"unifi|kerio"}'
    try:
        data = await loki.query_range(lq, limit=40)
        logs = _summarize("loki", data)
    except Exception as e:  # noqa: BLE001
        logs = f"(log oxunmadı: {e})"

    answer = await llm.generate(
        f"Problem: {question}\n\nRunbook/Knowledge:\n{knowledge}\n\nRecent live logs:\n{logs}",
        system=TROUBLESHOOT_SYSTEM,
    )
    sources = sorted({h.chunk.source for h in hits})
    return {
        "answer": answer.strip(), "source": "troubleshoot",
        "query": lq, "result": logs, "sources": sources,
    }


async def chat(question: str) -> dict:
    """Route and answer a question. Degrades gracefully when the LLM backend
    (Ollama) is unreachable or times out: the planner and every answer branch
    call the model, so a single guard here keeps /ai/chat from 500-ing the panel
    while the metric/log panels keep working. Prom/Loki failures are already
    handled inside the branches and surface as normal answers, not exceptions."""
    try:
        return await _run(question)
    except httpx.HTTPError as e:
        return {
            "answer": "AI köməkçi hazırda əlçatmazdır — model xidməti (Ollama) cavab "
            "vermir. Bir azdan yenidən cəhd edin. Metrik və log panelləri normal işləyir.",
            "source": "error",
            "query": "",
            "error": str(e),
        }


async def _run(question: str) -> dict:
    # 1) plan / route the question
    plan = await llm.generate_json(f"Sual: {question}", system=PLANNER_SYSTEM)
    source = plan.get("source", "")
    query = plan.get("query", "")

    # RAG branch: knowledge-base answer (how/why/docs).
    if source == "knowledge":
        return await _answer_knowledge(question, query)

    # Hybrid branch: runbook (RAG) + live logs for problem diagnosis.
    if source == "troubleshoot":
        return await _answer_troubleshoot(question, query, plan.get("logql", ""))

    err = _validate(source, query)
    if err:
        return {"answer": f"Sorğunu yarada bilmədim ({err}). Sualı bir az dəqiqləşdirin.", "source": source, "query": query}

    # 2) execute
    try:
        if source == "prometheus":
            data = await prom.query(query)
        else:
            data = await loki.query_range(query, limit=50)
    except Exception as e:  # noqa: BLE001
        return {"answer": f"Sorğu icra olunmadı: {e}", "source": source, "query": query}

    result_text = _summarize(source, data)

    # Named lookup that matched nothing: don't dead-end with "not found". The user
    # may have named a CLIENT while it is actually a DEVICE (e.g. asking about
    # "vadim" — really the AP "4.3.Vadim-AC-Lite"), or typed a person's name that
    # simply isn't in the data. Cross-check both client and device names for the
    # term and hand the answerer a concrete pointer.
    if source == "prometheus" and _is_empty_prom(data) and ("name=" in query or "name=~" in query):
        result_text += await _name_hint(query)

    # 3) answer from the result
    answer = await llm.generate(
        f"Sual: {question}\n\nİcra olunan {source} sorğusu: {query}\n\nNəticə:\n{result_text}",
        system=ANSWER_SYSTEM,
    )
    return {"answer": answer.strip(), "source": source, "query": query, "result": result_text}
