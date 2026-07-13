"""Phase-3 RAG: a tiny, fully-local knowledge base.

Corpus = curated markdown (runbooks + query examples) under app/knowledge/ plus a
live device-inventory snapshot pulled from Prometheus labels. Embeddings come from
Ollama (nomic-embed-text); retrieval is in-memory numpy cosine similarity with a
light lexical boost — no vector DB, no external services. The index is built once
(lazily, guarded by a lock) and kept in memory for the process lifetime; a restart
rebuilds it (fast for this corpus size).
"""

from __future__ import annotations

import asyncio
import re
from dataclasses import dataclass
from pathlib import Path

import numpy as np

from .clients import embedder, prom
from .config import settings

KNOWLEDGE_DIR = Path(__file__).parent / "knowledge"


@dataclass
class Chunk:
    text: str
    source: str   # file name or "canlı-inventar"
    section: str  # nearest heading


# --- chunking ---------------------------------------------------------------

def _windows(text: str, size: int = 1100, overlap: int = 150) -> list[str]:
    """Split long text into overlapping windows on paragraph/space boundaries."""
    text = text.strip()
    if len(text) <= size:
        return [text] if text else []
    out: list[str] = []
    i = 0
    while i < len(text):
        end = min(i + size, len(text))
        if end < len(text):
            # prefer to break on a paragraph, then a newline, then a space
            for sep in ("\n\n", "\n", " "):
                cut = text.rfind(sep, i + size - overlap, end)
                if cut != -1:
                    end = cut
                    break
        out.append(text[i:end].strip())
        if end <= i or end >= len(text):
            break
        nxt = end - overlap  # step back for overlap, but always make progress
        i = nxt if nxt > i else end
    return [w for w in out if w]


def _split_markdown(text: str, source: str) -> list[Chunk]:
    """Section a markdown doc by its headings; each chunk carries its heading so
    retrieval and citations stay meaningful."""
    chunks: list[Chunk] = []
    head = ""
    body: list[str] = []

    def flush() -> None:
        joined = "\n".join(body).strip()
        if not joined and not head:
            return
        full = f"{head}\n{joined}" if head else joined
        for piece in _windows(full):
            chunks.append(Chunk(text=piece, source=source, section=head or "—"))

    for line in text.splitlines():
        if re.match(r"^#{1,4}\s", line):
            flush()
            head = line.lstrip("#").strip()
            body = []
        else:
            body.append(line)
    flush()
    return chunks


def _load_markdown() -> list[Chunk]:
    if not KNOWLEDGE_DIR.is_dir():
        return []
    chunks: list[Chunk] = []
    for path in sorted(KNOWLEDGE_DIR.glob("*.md")):
        try:
            chunks += _split_markdown(path.read_text(encoding="utf-8"), path.name)
        except Exception:  # noqa: BLE001
            continue
    return chunks


async def _live_inventory() -> list[Chunk]:
    """Snapshot the real devices from Prometheus so the KB knows this network's
    actual APs/switches/gateways (names, models, IPs, state)."""
    try:
        data = await prom.query("unifi_device_info")
    except Exception:  # noqa: BLE001
        return []
    rows = data.get("data", {}).get("result", [])
    lines: list[str] = []
    for s in rows:
        m = s.get("metric", {})
        name = m.get("name") or m.get("mac", "?")
        lines.append(
            f"- {name}: tip={m.get('type','?')}, model={m.get('model','-')}, "
            f"vendor={m.get('vendor','?')}, ip={m.get('ip','-')}, status={m.get('state','?')}"
        )
    if not lines:
        return []
    chunks: list[Chunk] = []
    # ~12 devices per chunk keeps each chunk small and on-topic.
    for i in range(0, len(lines), 12):
        block = "Cihaz inventarı (canlı):\n" + "\n".join(lines[i : i + 12])
        chunks.append(Chunk(text=block, source="canlı-inventar", section="Cihaz inventarı"))
    return chunks


# --- retrieval --------------------------------------------------------------

_TOKEN = re.compile(r"[\wğüşıöçəĞÜŞİÖÇƏ]+", re.UNICODE)
# Fold Azerbaijani diacritics so lexical matching is robust to users typing
# without them (e.g. "yaddas"/"yuksek" should match "yaddaş"/"yüksək").
# Folding must run BEFORE .lower(): Python lowercases the dotted capital "İ" to
# "i" + U+0307 (combining dot above), which .translate can't reach afterwards, so
# "İnternet" would never match "internet". The third maketrans arg deletes any
# stray U+0307, and the uppercase entries map dotted/dotless capitals directly.
_FOLD = str.maketrans("əıöüçşğƏIİÖÜÇŞĞ", "eioucsgeiioucsg", "̇")


def _tokens(text: str) -> set[str]:
    return {t.translate(_FOLD).lower() for t in _TOKEN.findall(text) if len(t) > 2}


@dataclass
class Hit:
    chunk: Chunk
    score: float


class KnowledgeBase:
    def __init__(self) -> None:
        self.chunks: list[Chunk] = []
        self.matrix: np.ndarray | None = None  # (n, dim), L2-normalized rows
        self.token_sets: list[set[str]] = []
        self._lock = asyncio.Lock()
        self._built = False
        self.error: str | None = None

    @property
    def ready(self) -> bool:
        return self._built and bool(self.chunks)

    async def _compute(self) -> tuple[list[Chunk], np.ndarray | None, list[set[str]], str | None]:
        """Build the index data (markdown + live inventory + embeddings) WITHOUT
        mutating self, so callers can decide whether to commit the result. Returns
        (chunks, matrix, token_sets, error); error is set on any failure."""
        chunks = _load_markdown() + await _live_inventory()
        if not chunks:
            return [], None, [], "bilik bazası boşdur"
        try:
            vecs = await embedder.embed([c.text for c in chunks])
        except Exception as e:  # noqa: BLE001
            return [], None, [], f"embedding alınmadı: {e}"
        arr = np.asarray(vecs, dtype=np.float32)
        if arr.ndim != 2 or arr.shape[0] != len(chunks):
            return [], None, [], "embedding ölçüsü uyğunsuzdur"
        norms = np.linalg.norm(arr, axis=1, keepdims=True)
        norms[norms == 0] = 1.0
        return chunks, arr / norms, [_tokens(c.text) for c in chunks], None

    def _commit(self, chunks, matrix, token_sets, error) -> None:
        self.chunks, self.matrix, self.token_sets, self.error = chunks, matrix, token_sets, error
        self._built = True

    async def ensure_ready(self) -> None:
        if self._built:
            return
        async with self._lock:
            if self._built:
                return
            self._commit(*await self._compute())

    async def rebuild(self) -> None:
        """Force a fresh index: re-read knowledge/*.md and re-snapshot the live
        Prometheus inventory. Atomic — the new data is swapped in only if the
        rebuild succeeds, so a transient embedder/Prometheus outage during a
        periodic auto-reindex can't wipe an already-working index (we keep the old
        chunks and just record the error). asyncio.Lock isn't reentrant, so
        _compute must not re-acquire it — it doesn't touch self."""
        async with self._lock:
            chunks, matrix, token_sets, error = await self._compute()
            if error is None or not self._built:
                self._commit(chunks, matrix, token_sets, error)
            else:
                self.error = error  # keep the working index, note why refresh failed

    async def search(self, query: str, k: int | None = None) -> list[Hit]:
        await self.ensure_ready()
        if not self.ready or self.matrix is None:
            return []
        k = k or settings.rag_top_k
        try:
            qv = np.asarray((await embedder.embed([query]))[0], dtype=np.float32)
        except Exception:  # noqa: BLE001
            return []
        n = np.linalg.norm(qv) or 1.0
        cos = self.matrix @ (qv / n)  # (n,) cosine since rows are normalized

        # Light lexical boost: reward query-term overlap (cheap hybrid retrieval).
        qtok = _tokens(query)
        if qtok:
            lex = np.array(
                [len(qtok & ts) / len(qtok) for ts in self.token_sets], dtype=np.float32
            )
        else:
            lex = np.zeros_like(cos)
        scores = cos + 0.15 * lex

        idx = np.argsort(scores)[::-1][:k]
        return [Hit(self.chunks[i], float(scores[i])) for i in idx]


kb = KnowledgeBase()
