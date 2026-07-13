import asyncio
import logging
from contextlib import asynccontextmanager

import httpx
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel

from .config import settings
from .clients import prom, loki, llm, embedder
from . import agent
from . import insights
from .rag import kb

log = logging.getLogger("unico.ai")


async def _auto_reindex() -> None:
    """Keep the RAG index's live device inventory fresh without a restart. Builds
    once, then rebuilds every rag_reindex_seconds; rebuild() is atomic so a
    transient failure here never wipes a working index."""
    interval = settings.rag_reindex_seconds
    await kb.ensure_ready()
    if interval <= 0:
        return
    while True:
        await asyncio.sleep(interval)
        try:
            await kb.rebuild()
        except Exception:  # noqa: BLE001
            log.warning("RAG auto-reindex failed", exc_info=True)


@asynccontextmanager
async def lifespan(app: FastAPI):
    task = asyncio.create_task(_auto_reindex())
    try:
        yield
    finally:
        task.cancel()


app = FastAPI(title="Unico AI service", version="0.1.0", lifespan=lifespan)
# Locked down by default (no browser origin — the BFF calls this server-to-server).
# AI_CORS_ORIGINS can add an explicit allowlist; "*" is intentionally not honored.
_cors_origins = [o.strip() for o in settings.cors_origins.split(",") if o.strip() and o.strip() != "*"]
if _cors_origins:
    app.add_middleware(
        CORSMiddleware, allow_origins=_cors_origins, allow_methods=["*"], allow_headers=["*"]
    )


class ChatIn(BaseModel):
    question: str


@app.get("/ai/health")
async def health():
    out = {
        "model": settings.ollama_model,
        "embed_model": settings.embed_model,
        "ollama": False, "prometheus": False, "loki": False, "embed": False,
    }
    try:
        await llm.generate("ping", system="Reply with: ok")
        out["ollama"] = True
    except Exception:  # noqa: BLE001
        pass
    try:
        await prom.metric_names()
        out["prometheus"] = True
    except Exception:  # noqa: BLE001
        pass
    try:
        await loki.labels()
        out["loki"] = True
    except Exception:  # noqa: BLE001
        pass
    try:
        v = await embedder.embed(["ping"])
        out["embed"] = bool(v and v[0])
    except Exception:  # noqa: BLE001
        pass
    return out


@app.get("/ai/knowledge")
async def knowledge_status():
    """RAG index status: whether the knowledge base built and how many chunks."""
    await kb.ensure_ready()
    sources: dict[str, int] = {}
    for ch in kb.chunks:
        sources[ch.source] = sources.get(ch.source, 0) + 1
    return {"ready": kb.ready, "chunks": len(kb.chunks), "sources": sources, "error": kb.error}


@app.post("/ai/knowledge/reindex")
async def knowledge_reindex():
    """Force-rebuild the RAG index (re-reads the markdown corpus and re-snapshots
    the live Prometheus device inventory) without restarting the container."""
    await kb.rebuild()
    sources: dict[str, int] = {}
    for ch in kb.chunks:
        sources[ch.source] = sources.get(ch.source, 0) + 1
    return {"ready": kb.ready, "chunks": len(kb.chunks), "sources": sources, "error": kb.error}


@app.post("/ai/chat")
async def chat(body: ChatIn):
    return await agent.chat(body.question)


@app.get("/ai/insights")
async def ai_insights():
    """Phase-2 automatic analysis: anomalies + forecasts over unifi_* metrics,
    with a short Azerbaijani synthesis. Powers the Overview 'AI Insights' panel."""
    return await insights.compute()


@app.get("/ai/summary")
async def summary():
    """Phase-1 placeholder: a simple 'what happened' from recent error logs.
    Phase 2 will feed this from the anomaly/forecast insights store."""
    q = '{vendor=~"unifi|kerio"} | logfmt | level="error"'
    try:
        data = await loki.query_range(q, limit=100)
    except Exception as e:  # noqa: BLE001
        return {"summary": f"Loglar oxunmadı: {e}"}
    text = agent._summarize("loki", data)
    try:
        answer = await llm.generate(
            f"Recent error logs:\n{text}\n\nSummarize briefly in English: how many errors, "
            f"the most common one, which device/service, and a recommendation.",
            system="You are the Unico network monitoring assistant. Give a short, concrete summary in English.",
        )
    except httpx.HTTPError:
        return {"summary": "AI xülasəsi hazırda əlçatmazdır (model xidməti cavab vermir)."}
    return {"summary": answer.strip()}
