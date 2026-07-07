import json
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


prom = Prometheus(settings.prometheus_url)
loki = Loki(settings.loki_url)
llm = Ollama(settings.ollama_url, settings.ollama_model)
