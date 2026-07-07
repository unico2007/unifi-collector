from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """All config comes from env (prefix AI_), so nothing is hardcoded."""

    model_config = SettingsConfigDict(env_prefix="AI_", env_file=".env", extra="ignore")

    # Data sources (already running in the stack).
    prometheus_url: str = "http://prometheus:9090"
    loki_url: str = "http://loki:3100"

    # Local LLM via Ollama. Swap the model to scale up on a bigger GPU later.
    ollama_url: str = "http://host.docker.internal:11434"
    ollama_model: str = "llama3.2:3b"

    # Guardrails for LLM-generated queries.
    max_range: str = "24h"      # cap the time range the agent may query
    query_timeout: float = 15.0
    llm_timeout: float = 120.0


settings = Settings()
