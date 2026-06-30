"""HU-49.2: modelos Python (no ORM) usados por la logica del chat.

`Source` es lo que devuelve el retrieval y se persiste en `chat_messages.
sources` (JSONB). `RagContext` agrupa contexto + sources para construir
el prompt. Son DTOs inmutables.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Literal

SourceType = Literal["agent", "skill", "flow", "prompt", "project", "user", "usage"]


@dataclass(frozen=True)
class Source:
    """Una fuente citada en la respuesta del LLM.

    `score` es la similitud coseno (0..1). `snippet` es el texto del
    chunk truncado para mostrar en el frontend. `url` es la URL del
    admin donde ver el detalle del row origen.
    """

    table: str
    id: str
    title: str
    snippet: str
    score: float
    url: str = ""

    def to_dict(self) -> dict:
        return {
            "tabla": self.table,
            "id": self.id,
            "titulo": self.title,
            "snippet": self.snippet,
            "score": round(self.score, 4),
            "url": self.url,
        }

    @classmethod
    def from_dict(cls, d: dict) -> "Source":
        return cls(
            table=d.get("tabla") or d.get("table", ""),
            id=d.get("id", ""),
            title=d.get("titulo") or d.get("title", ""),
            snippet=d.get("snippet", ""),
            score=float(d.get("score", 0.0)),
            url=d.get("url", ""),
        )


@dataclass
class RagContext:
    """Resultado del retrieval listo para el prompt.

    `chunks` son los textos crudos a inyectar en el system prompt.
    `sources` son los Source serializables para persistir en metadata.
    Si `is_empty=True`, el LLM no tiene contexto y debe responder
    "no encontre informacion relevante".
    """

    chunks: list[str] = field(default_factory=list)
    sources: list[Source] = field(default_factory=list)
    is_empty: bool = True

    @property
    def top_score(self) -> float:
        return max((s.score for s in self.sources), default=0.0)