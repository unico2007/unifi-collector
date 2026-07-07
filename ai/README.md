# Unico AI service (Phase 1)

A small Python/FastAPI microservice that turns natural-language questions into
PromQL/LogQL, runs them against Prometheus + Loki, and answers in Azerbaijani
using a local LLM (Ollama). Later phases add anomaly detection, forecasting,
RAG and root-cause analysis behind the same service.

## Prerequisites (on the GPU box)
1. Install Ollama: https://ollama.com  → `ollama serve`
2. Pull a small model that fits ~4 GB VRAM:
   - `ollama pull llama3.2:3b`   (fast, real-time)
   - `ollama pull qwen2.5:7b`    (better at query/JSON; partial CPU offload)

## Run locally (dev)
```bash
cd ai
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
cp .env.example .env          # point at your Prometheus/Loki/Ollama
uvicorn app.main:app --reload --port 8090
```

## Endpoints
- `GET  /ai/health`  — checks Ollama / Prometheus / Loki connectivity
- `POST /ai/chat`    — `{"question":"CPU niyə yüksəlib?"}` → `{answer, source, query, result}`
- `GET  /ai/summary` — quick "what happened" from recent error logs

## Quick test
```bash
curl -s localhost:8090/ai/health | jq
curl -s -X POST localhost:8090/ai/chat -H 'content-type: application/json' \
  -d '{"question":"Neçə cihaz offline-dır?"}' | jq
```

## Config (env, prefix AI_)
`AI_PROMETHEUS_URL`, `AI_LOKI_URL`, `AI_OLLAMA_URL`, `AI_OLLAMA_MODEL`.
Swap `AI_OLLAMA_MODEL` to scale the LLM up on a bigger GPU later — no code change.
