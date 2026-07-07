"""Phase-1 query agent: natural language -> PromQL/LogQL -> answer.

Single-step tool use: the LLM plans ONE query (Prometheus or Loki), the backend
executes it (validated), then the LLM answers from the result. Multi-step
reasoning, forecasting and anomaly tools are added in later phases behind the
same interface.
"""

from .clients import prom, loki, llm

# Hand-written schema of the collector's neutral metrics — this grounds the LLM
# far better than raw metric names. Extend it as new vendors/metrics arrive.
SCHEMA = """
Prometheus metrics (namespace unifi_), labels in {}:
- unifi_device_up{vendor,site,mac,name,model,type}            1=online 0=offline
- unifi_device_cpu_percent{...}                               device CPU %
- unifi_device_memory_percent{...}                            device memory %
- unifi_device_uptime_seconds{...}
- unifi_device_rx_bytes{...} / unifi_device_tx_bytes{...}     cumulative counters (use rate())
- unifi_devices_total{vendor,site,type}                       device count
- unifi_clients_total{vendor,site}                            connected client count
- unifi_client_rssi{vendor,site,mac,name,ap,vlan}             signal dBm
- unifi_client_rx_rate / unifi_client_tx_rate{...}            bits/s
- unifi_health_status{vendor,site,subsystem}
vendor is "unifi" or "kerio".

Loki logs: labels {vendor,site,level,event}. Example stream selector {vendor="unifi"}.
Log levels: info, warn, error. Query with LogQL, e.g. {vendor="unifi"} |= "error".
""".strip()

PLANNER_SYSTEM = f"""You are a monitoring query planner for a UniFi + Kerio network.
Given a user question, output ONE query to answer it.
{SCHEMA}

Rules:
- Choose source "prometheus" for numbers/metrics/status, "loki" for log text/events.
- For rates/traffic use rate(metric[5m]).
- Keep any time range <= {'{'}max{'}'}.
- Output STRICT JSON: {{"source":"prometheus"|"loki","query":"...","reason":"..."}}
- Do NOT invent metric or label names outside the schema.
""".replace("{max}", "24h")

ANSWER_SYSTEM = """You are a helpful network monitoring assistant.
Answer the user's question in AZERBAIJANI, concisely and concretely, using ONLY
the query result provided. If the result is empty, say so plainly and suggest
what to check. Do not fabricate numbers."""


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


async def chat(question: str) -> dict:
    # 1) plan the query
    plan = await llm.generate_json(f"Sual: {question}", system=PLANNER_SYSTEM)
    source = plan.get("source", "")
    query = plan.get("query", "")

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

    # 3) answer from the result
    answer = await llm.generate(
        f"Sual: {question}\n\nİcra olunan {source} sorğusu: {query}\n\nNəticə:\n{result_text}",
        system=ANSWER_SYSTEM,
    )
    return {"answer": answer.strip(), "source": source, "query": query, "result": result_text}
