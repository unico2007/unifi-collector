from __future__ import annotations

import json
import re
import httpx
from .config import settings


class Prometheus:
    def __init__(self, url: str):
        self.url = url.rstrip("/")

    async def query(self, promql: str) -> dict:
        async with httpx.AsyncClient(timeout=settings.query_timeout) as c:
            r = await c.get(f"{self.url}/api/v1/query", params={"query": promql})
            r.raise_for_status()
            return r.json()

    async def query_range(self, promql: str, start: str, end: str, step: str) -> dict:
        async with httpx.AsyncClient(timeout=settings.query_timeout) as c:
            r = await c.get(
                f"{self.url}/api/v1/query_range",
                params={"query": promql, "start": start, "end": end, "step": step},
            )
            r.raise_for_status()
            return r.json()

    async def metric_names(self) -> list[str]:
        async with httpx.AsyncClient(timeout=settings.query_timeout) as c:
            r = await c.get(f"{self.url}/api/v1/label/__name__/values")
            r.raise_for_status()
            return r.json().get("data", [])


class Loki:
    def __init__(self, url: str):
        self.url = url.rstrip("/")

    async def query_range(self, logql: str, limit: int = 50) -> dict:
        async with httpx.AsyncClient(timeout=settings.query_timeout) as c:
            r = await c.get(
                f"{self.url}/loki/api/v1/query_range",
                params={"query": logql, "limit": limit, "direction": "backward"},
            )
            r.raise_for_status()
            return r.json()

    async def labels(self) -> list[str]:
        async with httpx.AsyncClient(timeout=settings.query_timeout) as c:
            r = await c.get(f"{self.url}/loki/api/v1/labels")
            r.raise_for_status()
            return r.json().get("data", [])


class Ollama:
    def __init__(self, url: str, model: str):
        self.url = url.rstrip("/")
        self.model = model

    async def generate(self, prompt: str, system: str = "", fmt: str | None = None) -> str:
        payload: dict = {
            "model": self.model,
            "prompt": prompt,
            "system": system,
            "stream": False,
            "options": {"temperature": 0.1},
        }
        if fmt == "json":
            payload["format"] = "json"
        async with httpx.AsyncClient(timeout=settings.llm_timeout) as c:
            r = await c.post(f"{self.url}/api/generate", json=payload)
            r.raise_for_status()
            return r.json().get("response", "")

    async def generate_json(self, prompt: str, system: str = "") -> dict:
        raw = await self.generate(prompt, system, fmt="json")
        try:
            return json.loads(raw)
        except json.JSONDecodeError:
            return {}

    async def embed(self, texts: list[str]) -> list[list[float]]:
        """Batch-embed texts via Ollama /api/embed (used for RAG). Returns one
        vector per input. Raises on transport/HTTP errors so callers can degrade."""
        if not texts:
            return []
        async with httpx.AsyncClient(timeout=settings.llm_timeout) as c:
            r = await c.post(
                f"{self.url}/api/embed",
                json={"model": self.model, "input": texts},
            )
            r.raise_for_status()
            return r.json().get("embeddings", [])


class OpenAICompat:
    """Chat client for any OpenAI-compatible endpoint. Used for NVIDIA NIM
    (base_url=https://integrate.api.nvidia.com/v1). Mirrors the Ollama method
    surface (generate / generate_json) so agent.py is provider-agnostic."""

    def __init__(self, base_url: str, api_key: str, model: str, timeout: float):
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.model = model
        self.timeout = timeout

    async def generate(self, prompt: str, system: str = "", fmt: str | None = None) -> str:
        messages = []
        if system:
            messages.append({"role": "system", "content": system})
        # Qwen3 "thinking" models spend most of the latency emitting a long
        # chain-of-thought before the answer. Routing + short factual answers here
        # need no visible reasoning, so use Qwen3's documented soft switch
        # `/no_think` to disable it — a big speed-up. Harmless text otherwise.
        user = prompt + " /no_think" if "qwen3" in self.model.lower() else prompt
        messages.append({"role": "user", "content": user})
        payload: dict = {
            "model": self.model,
            "messages": messages,
            "temperature": 0.1,
            "max_tokens": 1024,
            "stream": False,
        }
        if fmt == "json":
            # Force clean JSON so the planner parse never trips on prose/reasoning.
            payload["response_format"] = {"type": "json_object"}
        # Gemini 2.5 models "think" by default, which dominates latency for our
        # routing + short-answer workload. Google's OpenAI-compat layer disables
        # it via reasoning_effort="none". (Harmless/ignored on non-Gemini models.)
        if "gemini" in self.model.lower():
            payload["reasoning_effort"] = "none"
        headers = {"Authorization": f"Bearer {self.api_key}"}
        async with httpx.AsyncClient(timeout=self.timeout) as c:
            r = await c.post(f"{self.base_url}/chat/completions", json=payload, headers=headers)
            r.raise_for_status()
            data = r.json()
            content = data["choices"][0]["message"]["content"] or ""
            # Strip any residual (usually empty) <think>…</think> block /no_think leaves.
            return re.sub(r"^\s*<think>.*?</think>\s*", "", content, flags=re.DOTALL)

    async def generate_json(self, prompt: str, system: str = "") -> dict:
        raw = await self.generate(prompt, system, fmt="json")
        try:
            return json.loads(raw)
        except json.JSONDecodeError:
            return {}


class FallbackLLM:
    """Try the primary model (cloud), fall back to the local one on any transport
    error (API down / timeout / HTTP error), so the assistant keeps working even
    when the internet or NVIDIA is unavailable."""

    def __init__(self, primary, fallback):
        self.primary = primary
        self.fallback = fallback

    async def generate(self, prompt: str, system: str = "", fmt: str | None = None) -> str:
        try:
            return await self.primary.generate(prompt, system, fmt)
        except httpx.HTTPError:
            return await self.fallback.generate(prompt, system, fmt)

    async def generate_json(self, prompt: str, system: str = "") -> dict:
        try:
            return await self.primary.generate_json(prompt, system)
        except httpx.HTTPError:
            return await self.fallback.generate_json(prompt, system)


prom = Prometheus(settings.prometheus_url)
loki = Loki(settings.loki_url)

# Local Ollama is always constructed (it is also the fallback + embedder host).
_ollama = Ollama(settings.ollama_url, settings.ollama_model)
# Provider priority: Gemini > NVIDIA > local Ollama. Whichever cloud key is set
# becomes the primary model; local Ollama is always the automatic fallback.
if settings.gemini_api_key:
    _gemini = OpenAICompat(
        settings.gemini_base_url, settings.gemini_api_key, settings.gemini_model, settings.gemini_timeout
    )
    llm = FallbackLLM(primary=_gemini, fallback=_ollama)
elif settings.nvidia_api_key:
    _nim = OpenAICompat(
        settings.nvidia_base_url, settings.nvidia_api_key, settings.nvidia_model, settings.nvidia_timeout
    )
    llm = FallbackLLM(primary=_nim, fallback=_ollama)
else:
    llm = _ollama

# Embeddings stay local (nomic-embed-text): cheap, keeps RAG source text on-box.
embedder = Ollama(settings.ollama_url, settings.embed_model)
