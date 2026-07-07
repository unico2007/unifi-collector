from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel

from .config import settings
from .clients import prom, loki, llm
from . import agent
from . import insights

app = FastAPI(title="Unico AI service", version="0.1.0")
app.add_middleware(
    CORSMiddleware, allow_origins=["*"], allow_methods=["*"], allow_headers=["*"]
)


class ChatIn(BaseModel):
    question: str


@app.get("/ai/health")
async def health():
    out = {"model": settings.ollama_model, "ollama": False, "prometheus": False, "loki": False}
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
    return out


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
    answer = await llm.generate(
        f"Son error logları:\n{text}\n\nBunları Azərbaycanca qısa xülasə et: neçə error, "
        f"ən çox hansı, hansı cihaz/servis, tövsiyə.",
        system="Sən şəbəkə monitorinq köməkçisisən. Qısa, konkret xülasə ver.",
    )
    return {"summary": answer.strip()}
