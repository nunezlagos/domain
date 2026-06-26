"""HU-49.1: paquete de abstraccion LLM.

Centraliza la seleccion y configuracion de providers LLM (Anthropic,
MiniMax). La interfaz LlmProvider vive en `provider`; las implementaciones
concretas en archivos separados. `LlmFactory` resuelve el provider segun
env vars (LLM_PROVIDER). Usado por HU-49.2 (ChatService) y por futuras
HU de generacion AI (prompt generation, summary, etc).
"""
from .factory import LlmFactory, get_default_provider
from .provider import LlmProvider, LlmProviderError
from .types import ChatMessage, ChatRequest, ChatResponse, ChatUsage

__all__ = [
    "LlmFactory",
    "LlmProvider",
    "LlmProviderError",
    "ChatMessage",
    "ChatRequest",
    "ChatResponse",
    "ChatUsage",
    "get_default_provider",
]