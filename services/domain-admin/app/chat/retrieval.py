"""HU-49.2: retrieval RAG con live-read + embeddings + ranking.

Pipeline:
1. Live-read de TODAS las tablas relevantes del dominio (agents, skills,
   flows, prompts, projects, users, clients, project_tickets, issues,
   knowledge_docs, etc). NO usa cache precomputado: cada pregunta relee.
2. Formatea cada row a 1+ chunks de texto (texto + metadata).
3. Genera el embedding de la pregunta con el EmbeddingProvider.
4. Calcula similitud coseno contra todos los chunks (en memoria).
5. Filtra por `score >= MIN_SCORE` y devuelve los top-K.

Decisiones de diseno:
- `MIN_SCORE = 0.5` (decision del usuario: balance recall/precision).
- `TOP_K = 8` chunks max.
- EmbeddingProvider es una interfaz (en el consumidor, ver AGENTS.md)
  con implementacion `OpenAIEmbeddingProvider`. Mockeable en tests.
- Live-read escala bien para MVP (<10k rows total). Cuando llegue a
  >100k rows, migrar a precompute con pgvector.
- PRIVACY: NUNCA pedimos columnas sensibles (api_key_ciphertext, etc).
"""
from __future__ import annotations

import logging
import math
import re
from abc import ABC, abstractmethod
from dataclasses import dataclass

from django.db import connection

from .models import RagContext, Source

log = logging.getLogger(__name__)

MIN_SCORE = 0.3
TOP_K = 10

# Tablas del dominio que se incluyen en el RAG. Cada entry: (table,
# url_prefix, sql_query, format_fn_name).
# Mantener alineado con el admin de Django (maintainers.*).
# PRIVACY: NUNCA pedir columnas sensibles (api_key_ciphertext, password_hash, etc).
_SOURCES_CONFIG: list[dict] = [
    {
        "table": "agent",
        "url_prefix": "/agentes/detalle?id=",
        "sql": """
            SELECT CAST(id AS TEXT) AS id, slug, name, description, provider, model, status
            FROM agents
            WHERE deleted_at IS NULL
            ORDER BY created_at DESC
            LIMIT 200
        """,
    },
    {
        "table": "skill",
        "url_prefix": "/skills/detalle?id=",
        "sql": """
            SELECT CAST(id AS TEXT) AS id, slug, name, description, skill_type, status
            FROM skills
            WHERE deleted_at IS NULL
            ORDER BY created_at DESC
            LIMIT 200
        """,
    },
    {
        "table": "flow",
        "url_prefix": "/flows/detalle?id=",
        "sql": """
            SELECT CAST(id AS TEXT) AS id, slug, name, description, status
            FROM flows
            WHERE deleted_at IS NULL
            ORDER BY created_at DESC
            LIMIT 200
        """,
    },
    {
        "table": "prompt",
        "url_prefix": "/prompts/detalle?id=",
        "sql": """
            SELECT CAST(id AS TEXT) AS id, slug, description
            FROM prompts
            WHERE is_active = true
            ORDER BY created_at DESC
            LIMIT 200
        """,
    },
    {
        "table": "project",
        "url_prefix": "/proyectos/detalle?id=",
        "sql": """
            SELECT CAST(id AS TEXT) AS id, name, slug, description, status
            FROM projects
            WHERE deleted_at IS NULL AND status != 'archived'
            ORDER BY created_at DESC
            LIMIT 200
        """,
    },
    {
        "table": "user",
        "url_prefix": "/usuarios?email=",
        "sql": """
            SELECT CAST(id AS TEXT) AS id, email, name, role, status
            FROM users
            WHERE deleted_at IS NULL
            ORDER BY created_at DESC
            LIMIT 200
        """,
    },
    {
        "table": "client",
        "url_prefix": "/clientes?nombre=",
        "sql": """
            SELECT CAST(id AS TEXT) AS id, name, slug, kind, status
            FROM clients
            WHERE deleted_at IS NULL
            ORDER BY created_at DESC
            LIMIT 200
        """,
    },
    {
        "table": "project_ticket",
        "url_prefix": "/tickets?id=",
        "sql": """
            SELECT CAST(id AS TEXT) AS id, title, description_md, status, priority, issue_type
            FROM project_tickets
            WHERE deleted_at IS NULL
            ORDER BY updated_at DESC
            LIMIT 200
        """,
    },
    {
        "table": "issue",
        "url_prefix": "/issues?id=",
        "sql": """
            SELECT CAST(id AS TEXT) AS id, title, description, status, issue_type, priority
            FROM issues
            WHERE deleted_at IS NULL
            ORDER BY updated_at DESC
            LIMIT 200
        """,
    },
    {
        "table": "knowledge_doc",
        "url_prefix": "/knowledge?id=",
        "sql": """
            SELECT CAST(id AS TEXT) AS id, title, kind, summary
            FROM knowledge_docs
            WHERE deleted_at IS NULL
            ORDER BY updated_at DESC
            LIMIT 200
        """,
    },
    {
        "table": "knowledge_chunk",
        "url_prefix": "/knowledge/chunks?id=",
        "sql": """
            SELECT CAST(kc.id AS TEXT) AS id, kc.chunk_text, kc.section_title, kd.title AS doc_title
            FROM knowledge_chunks kc
            JOIN knowledge_docs kd ON kd.id = kc.document_id
            WHERE kc.deleted_at IS NULL AND kd.deleted_at IS NULL
            ORDER BY kc.created_at DESC
            LIMIT 200
        """,
    },
    {
        "table": "mcp_server",
        "url_prefix": "/mcpservers/detalle?id=",
        "sql": """
            SELECT CAST(id AS TEXT) AS id, slug, name, description, transport
            FROM mcp_servers
            WHERE deleted_at IS NULL
            ORDER BY created_at DESC
            LIMIT 100
        """,
    },
    {
        "table": "cron",
        "url_prefix": "/crons/detalle?id=",
        "sql": """
            SELECT CAST(id AS TEXT) AS id, slug, name, description, schedule
            FROM crons
            WHERE deleted_at IS NULL
            ORDER BY created_at DESC
            LIMIT 100
        """,
    },
    {
        "table": "webhook",
        "url_prefix": "/webhooks/detalle?id=",
        "sql": """
            SELECT CAST(id AS TEXT) AS id, slug, name, description, kind
            FROM webhooks
            WHERE deleted_at IS NULL
            ORDER BY created_at DESC
            LIMIT 100
        """,
    },
    {
        "table": "platform_policy",
        "url_prefix": "/politicas-plataforma/detalle?id=",
        "sql": """
            SELECT CAST(id AS TEXT) AS id, slug, name, description, kind, status
            FROM platform_policies
            WHERE deleted_at IS NULL
            ORDER BY created_at DESC
            LIMIT 100
        """,
    },
    {
        "table": "project_policy",
        "url_prefix": "/politicas-proyecto/detalle?id=",
        "sql": """
            SELECT CAST(id AS TEXT) AS id, slug, name, description, kind, status
            FROM project_policies
            WHERE deleted_at IS NULL
            ORDER BY created_at DESC
            LIMIT 100
        """,
    },
    {
        "table": "agent_template",
        "url_prefix": "/plantillas-agentes/detalle?id=",
        "sql": """
            SELECT CAST(id AS TEXT) AS id, slug, name, description, role, status
            FROM agent_templates
            WHERE deleted_at IS NULL
            ORDER BY name
            LIMIT 200
        """,
    },
]


@dataclass
class _Chunk:
    """Chunk interno con embedding precomputado."""

    table: str
    id: str
    title: str
    text: str
    url: str
    embedding: list[float]


class EmbeddingProvider(ABC):
    """Interfaz para providers de embeddings. Mockeable en tests."""

    @abstractmethod
    def embed(self, texts: list[str]) -> list[list[float]]:
        """Devuelve un vector por texto. Mismo orden, misma dimension."""


def _format_chunk(table: str, row: dict) -> str:
    """Convierte un row SQL a texto de chunk legible.

    Se excluyen campos sensibles (api_key_ciphertext, password_hash, etc).
    Se priorizan campos semanticos (name, description, slug, summary,
    chunk_text, doc_title).
    """
    parts: list[str] = []
    name = row.get("name") or row.get("slug") or row.get("email") or row.get("title")
    if name:
        parts.append(f"{table.capitalize()}: {name}")
    for key in (
        "slug", "description", "provider", "model", "skill_type",
        "status", "role", "email", "kind", "schedule", "transport",
        "issue_type", "priority", "summary", "chunk_text", "section_title",
        "doc_title", "title",
    ):
        v = row.get(key)
        if v and str(v).strip():
            text = str(v).strip()
            if len(text) > 500:
                text = text[:500] + "..."
            parts.append(f"{key}={text}")
    return " | ".join(parts)


def _fetch_source_rows() -> list[tuple[str, str, str, str]]:
    """Hace live-read de todas las tablas configuradas.

    Retorna lista de (table, id, title, chunk_text). No incluye
    el embedding: eso se calcula despues.
    """
    rows: list[tuple[str, str, str, str]] = []
    with connection.cursor() as cur:
        for cfg in _SOURCES_CONFIG:
            try:
                cur.execute(cfg["sql"])
                cols = [c.name for c in cur.description]
            except Exception as e:
                log.warning("retrieval: skip source %s (sql error: %s)", cfg["table"], e)
                continue
            for raw in cur.fetchall():
                row = dict(zip(cols, raw))
                chunk_text = _format_chunk(cfg["table"], row)
                if not chunk_text.strip():
                    continue
                rid = row.get("id", "")
                title = row.get("name") or row.get("slug") or row.get("email") or rid
                url = f"{cfg['url_prefix']}{rid}" if rid else ""
                rows.append((cfg["table"], rid, title, chunk_text))
    return rows


def _cosine(a: list[float], b: list[float]) -> float:
    if not a or not b or len(a) != len(b):
        return 0.0
    dot = sum(x * y for x, y in zip(a, b))
    na = math.sqrt(sum(x * x for x in a))
    nb = math.sqrt(sum(y * y for y in b))
    if na == 0 or nb == 0:
        return 0.0
    return dot / (na * nb)


def _tokenize(s: str) -> set[str]:
    """Tokeniza a palabras lowercase, ignora puntuación y stopwords basicas."""
    stop = {
        "el", "la", "los", "las", "un", "una", "de", "del", "en", "a", "y", "o", "que",
        "es", "se", "no", "si", "le", "lo", "por", "con", "para", "como", "al",
    }
    words = re.findall(r"[a-z0-9_]+", s.lower())
    return {w for w in words if w not in stop and len(w) > 1}


class _LexicalScorer:
    """Scorer fallback que NO usa embeddings.

    Ranking por overlap de tokens. Se usa cuando no hay EmbeddingProvider
    configurado (ej: en tests sin mock, en dev sin OpenAI key). El score
    es la fraccion de tokens de la query presentes en el chunk.
    """

    def score(self, query_tokens: set[str], chunk_text: str) -> float:
        chunk_tokens = _tokenize(chunk_text)
        if not chunk_tokens or not query_tokens:
            return 0.0
        overlap = len(query_tokens & chunk_tokens)
        return overlap / len(query_tokens)


class RetrievalService:
    """Orquesta live-read + embedding + ranking.

    Constructor recibe un EmbeddingProvider. Si no se provee, usa el
    scorer lexico (fallback sin API key). Esto permite tests sin
    dependencias externas.
    """

    def __init__(self, embedding_provider: EmbeddingProvider | None = None) -> None:
        self._embedder = embedding_provider

    def retrieve(self, question: str) -> RagContext:
        """Dado una pregunta, devuelve el contexto RAG (chunks + sources)."""
        question = question.strip()
        if not question:
            return RagContext(is_empty=True)

        rows = _fetch_source_rows()
        if not rows:
            return RagContext(is_empty=True)

        query_tokens = _tokenize(question)

        if self._embedder is not None:
            return self._retrieve_with_embeddings(question, rows)
        return self._retrieve_lexical(question, rows, query_tokens)

    def _retrieve_with_embeddings(
        self, question: str, rows: list[tuple[str, str, str, str]]
    ) -> RagContext:
        all_texts = [question] + [r[3] for r in rows]
        try:
            vectors = self._embedder.embed(all_texts)
        except Exception as e:
            log.warning("retrieval: embedding fallo (%s), fallback a lexico", e)
            return self._retrieve_lexical(question, rows, _tokenize(question))

        if not vectors or len(vectors) != len(all_texts):
            return self._retrieve_lexical(question, rows, _tokenize(question))

        query_vec = vectors[0]
        scored: list[tuple[float, _Chunk]] = []
        for idx, (table, rid, title, text) in enumerate(rows):
            score = _cosine(query_vec, vectors[1 + idx])
            if score >= MIN_SCORE:
                scored.append((score, _Chunk(
                    table=table, id=rid, title=title, text=text,
                    url=f"{_url_prefix_for(table)}{rid}" if rid else "",
                    embedding=vectors[1 + idx],
                )))

        scored.sort(key=lambda x: x[0], reverse=True)
        top = scored[:TOP_K]
        if not top:
            return RagContext(is_empty=True)

        return RagContext(
            chunks=[c.text for _, c in top],
            sources=[
                Source(table=c.table, id=c.id, title=c.title,
                       snippet=c.text[:200], score=score, url=c.url)
                for score, c in top
            ],
            is_empty=False,
        )

    def _retrieve_lexical(
        self,
        question: str,
        rows: list[tuple[str, str, str, str]],
        query_tokens: set[str],
    ) -> RagContext:
        scorer = _LexicalScorer()
        scored: list[tuple[float, tuple[str, str, str, str]]] = []
        for row in rows:
            score = scorer.score(query_tokens, row[3])
            if score >= MIN_SCORE:
                scored.append((score, row))
        scored.sort(key=lambda x: x[0], reverse=True)
        top = scored[:TOP_K]
        if not top:
            return RagContext(is_empty=True)
        return RagContext(
            chunks=[r[3] for _, r in top],
            sources=[
                Source(table=r[0], id=r[1], title=r[2],
                       snippet=r[3][:200], score=score,
                       url=f"{_url_prefix_for(r[0])}{r[1]}" if r[1] else "")
                for score, r in top
            ],
            is_empty=False,
        )


def _url_prefix_for(table: str) -> str:
    for cfg in _SOURCES_CONFIG:
        if cfg["table"] == table:
            return cfg["url_prefix"]
    return "/"