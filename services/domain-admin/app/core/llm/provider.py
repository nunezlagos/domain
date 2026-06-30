"""HU-49.1: interfaz abstracta del provider LLM.

Las interfaces viven en el consumidor (regla del AGENTS.md: "interfaces se
definen en el consumidor, no junto a la implementacion"). El consumidor
principal es ChatService (HU-49.2); a futuro tambien prompt generation,
summary, etc.

Hoy la interfaz tiene 1 metodo (chat_complete). Streaming NO se incluye
en el MVP (polling + render completo basta para NotebookLM). Cuando se
sume SSE en HU-49.4, se agrega `chat_stream(Iterator[ChatChunk])` aca.
"""
from __future__ import annotations

from abc import ABC, abstractmethod

from .types import ChatRequest, ChatResponse


class LlmProviderError(RuntimeError):
    """Error al comunicarse con el provider LLM (red, auth, rate limit)."""


class LlmProvider(ABC):
    """Interfaz para providers LLM (Anthropic, MiniMax, futuros)."""

    @abstractmethod
    def chat_complete(self, request: ChatRequest) -> ChatResponse:
        """Envia una conversacion al LLM y devuelve la respuesta completa.

        Raises:
            LlmProviderError: si falla la llamada (red, auth, timeout, etc).
        """