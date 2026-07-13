from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """All config comes from env (prefix AI_), so nothing is hardcoded."""

    model_config = SettingsConfigDict(env_prefix="AI_", env_file=".env", extra="ignore")

    # Data sources (already running in the stack).
    prometheus_url: str = "http://prometheus:9090"
    loki_url: str = "http://loki:3100"

    # Local LLM via Ollama. Swap the model to scale up on a bigger GPU later.
    ollama_url: str = "http://host.docker.internal:11434"
    # qwen2.5:7b fits a 6 GB GPU and is much better at Azerbaijani + PromQL/JSON.
    # Fall back to llama3.2:3b on a smaller GPU via AI_OLLAMA_MODEL.
    ollama_model: str = "qwen2.5:7b"

    # Cloud LLM via NVIDIA NIM (OpenAI-compatible, free tier). When
    # AI_NVIDIA_API_KEY is set, it becomes the PRIMARY model and the local Ollama
    # model above stays as an automatic fallback if the API is unreachable. Empty
    # key => local-only (unchanged behaviour). A big Qwen3 handles Azerbaijani far
    # better than the local 7B, so answers can be in Azerbaijani (see answer_lang).
    nvidia_api_key: str = ""
    nvidia_base_url: str = "https://integrate.api.nvidia.com/v1"
    nvidia_model: str = "qwen/qwen3.5-397b-a17b"
    nvidia_timeout: float = 60.0

    # Cloud LLM via Google Gemini (OpenAI-compatible endpoint). When
    # AI_GEMINI_API_KEY is set it becomes the PRIMARY model (takes priority over
    # NVIDIA), with local Ollama as the automatic fallback. Gemini 2.5 Flash is
    # fast (~1-3s) and strong at Azerbaijani. Works with a free or paid key — same
    # code, only the key differs. Never commit the key.
    gemini_api_key: str = ""
    gemini_base_url: str = "https://generativelanguage.googleapis.com/v1beta/openai"
    gemini_model: str = "gemini-3.1-flash-lite"
    gemini_timeout: float = 40.0

    # Language the assistant answers in. Empty = auto: Azerbaijani when the cloud
    # model is active (nvidia_api_key set), else English (the local 7B is weak at
    # Azerbaijani). Set AI_ANSWER_LANG explicitly to override. Planner stays
    # English/JSON regardless.
    answer_lang: str = ""
    # Local embedding model for RAG (Phase 3). Pull once on the host:
    #   ollama pull nomic-embed-text
    # 768-dim, ~275 MB, fast — coexists with qwen2.5 on the 6 GB GPU.
    embed_model: str = "nomic-embed-text"

    # RAG retrieval (Phase 3).
    rag_top_k: int = 4          # chunks fed to the answerer
    rag_min_score: float = 0.35  # below this, treat as "no relevant knowledge"
    # Periodically re-snapshot the live device inventory into the RAG index so it
    # doesn't go stale as devices change (no restart/manual reindex needed).
    # 0 disables the background refresh. Default 30 min.
    rag_reindex_seconds: int = 1800

    # CORS. The service is reached server-to-server by the BFF (no browser
    # origin), so the default is "no cross-origin browser access". Set
    # AI_CORS_ORIGINS to a comma-separated allowlist only if a browser must call
    # it directly. Never use "*" — it let any internet page script this API.
    cors_origins: str = ""

    # Lazy TTL cache for the Overview "AI Insights" panel. The panel polls per
    # browser, and each compute costs an LLM call — without this, N open tabs =
    # N calls/minute, which blows small cloud free-tier daily quotas. With it the
    # LLM runs at most once per this window regardless of viewers, and only when
    # someone is actually looking (lazy). 0 disables caching.
    insights_cache_seconds: int = 300

    # Guardrails for LLM-generated queries.
    max_range: str = "24h"      # cap the time range the agent may query
    query_timeout: float = 15.0
    llm_timeout: float = 120.0


settings = Settings()
