"""Phase-1 query agent: natural language -> PromQL/LogQL -> answer.

Single-step tool use: the LLM plans ONE query (Prometheus or Loki), the backend
executes it (validated), then the LLM answers from the result. Multi-step
reasoning, forecasting and anomaly tools are added in later phases behind the
same interface.
"""

from __future__ import annotations

from .clients import prom, loki, llm
from .config import settings
from .rag import kb

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

PLANNER_SYSTEM = f"""You are a monitoring query planner+router for a UniFi + Kerio network.
Given a user question, decide how to answer it.
{SCHEMA}

Pick ONE source:
- "prometheus" — for live numbers/metrics/status (counts, CPU, memory, rssi, traffic).
- "loki" — for actual log text/events right now.
- "knowledge" — for how-to / why / "what does X mean" / troubleshooting steps /
  network layout / procedures / documentation. No live number is needed; the answer
  comes from the knowledge base. For "knowledge", put good search keywords in "query".

Rules:
- For rates/traffic use rate(metric[5m]).
- Keep any time range <= {'{'}max{'}'}.
- Output STRICT JSON: {{"source":"prometheus"|"loki"|"knowledge","query":"...","reason":"..."}}
- Do NOT invent metric or label names outside the schema.
""".replace("{max}", "24h")

# Shared Azerbaijani style guide. qwen2.5:7b tends to drift into Turkish
# vocabulary/grammar for Azerbaijani; this glossary of "use X not Y" pairs plus
# the "keep technical terms" rule sharply improves the output quality.
AZ_STYLE = """Cavabı DÜZGÜN və TƏBİİ AZƏRBAYCAN DİLİNDƏ yaz (Türkiyə türkcəsi YOX).
- Türkcə söz/qrammatika işlətmə. Düzgün Azərbaycanca qarşılıqlar:
  server (yox: sunucu), istifadə (yox: kullanım), faiz (yox: yüzde),
  qoşulub (yox: bağlanmış/bağlı), vəziyyət (yox: durum), göstərir (yox: gösterir),
  səviyyə (yox: seviye), tədbir gör (yox: önlem al), yoxla (yox: kontrol et),
  diqqət (yox: dikkat), yüksək (yox: yüksek), mümkün (yox: olası).
- Texniki terminləri olduğu kimi saxla: CPU, RAM, IP, MAC, offline, online, AP,
  switch, gateway, VLAN, RSSI, PromQL, LogQL, dBm, Mbps, uplink."""

ANSWER_SYSTEM = f"""Sən Unico şəbəkə monitorinq köməkçisisən. İstifadəçinin sualına qısa və konkret cavab ver.
{AZ_STYLE}
- YALNIZ verilən sorğu nəticəsindən istifadə et; rəqəm uydurma. Nəticə boşdursa, bunu açıq de və nəyi yoxlamağı təklif et.
- Cavab 1-3 cümlə olsun; lazım olsa qısa maddələr işlət.

Nümunə:
Sual: Neçə cihaz offline-dır?
Nəticə: 3 cihaz offline.
Cavab: Hazırda 3 cihaz offline-dır. Həmin cihazların elektrik qidasını və uplink bağlantısını yoxlayın."""

KNOWLEDGE_SYSTEM = f"""Sən Unico şəbəkə monitorinq köməkçisisən. İstifadəçiyə qısa və konkret cavab ver. YALNIZ aşağıdakı "Bilik" mətninə əsaslan; orada olmayanı uydurma. Uyğun məlumat yoxdursa, açıq de və nəyi yoxlamağı təklif et. Addımlar varsa nömrələ. İstinad etdiyin mənbəni sonda göstər (məs. mənbə: runbooks.md).
{AZ_STYLE}"""


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


async def chat(question: str) -> dict:
    # 1) plan / route the question
    plan = await llm.generate_json(f"Sual: {question}", system=PLANNER_SYSTEM)
    source = plan.get("source", "")
    query = plan.get("query", "")

    # RAG branch: knowledge-base answer (how/why/troubleshooting/docs).
    if source == "knowledge":
        return await _answer_knowledge(question, query)

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
