"""HU-49.1: tipos de datos para el chat LLM.

Dataclasses inmutables (frozen=True salvo cuando hay listas). Usadas como
DTO entre el RetrievalService, ChatService y los providers concretos.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Literal

Role = Literal["user", "assistant", "system"]


@dataclass(frozen=True)
class ChatMessage:
    """Un mensaje individual en una conversacion.

    `role` sigue la convencion Anthropic/OpenAI. `content` es texto plano;
    para multimodal (imagenes) se usaria una variante ChatContentPart en una
    HU futura.
    """

    role: Role
    content: str


@dataclass(frozen=True)
class ChatRequest:
    """Payload de entrada para LlmProvider.chat_complete.

    `system` se separa de messages porque Anthropic lo trata como parametro
    independiente. `max_tokens` y `temperature` tienen defaults razonables.
    """

    messages: list[ChatMessage]
    system: str = ""
    model: str = ""
    max_tokens: int = 2048
    temperature: float = 0.3


@dataclass(frozen=True)
class ChatUsage:
    """Conteo de tokens devuelto por el provider."""

    input_tokens: int = 0
    output_tokens: int = 0


@dataclass(frozen=True)
class ChatResponse:
    """Respuesta completa del LLM."""

    content: str
    model: str
    usage: ChatUsage = field(default_factory=ChatUsage)
    stop_reason: str = "end_turn"