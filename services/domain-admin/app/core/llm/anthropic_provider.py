"""HU-49.1: implementacion Anthropic-compatible (provider base).

Soporta el Anthropic SDK oficial apuntando a cualquier endpoint compatible:
- https://api.anthropic.com (Anthropic directo)
- https://api.minimax.io/anthropic (MiniMax internacional)
- https://api.minimaxi.com/anthropic (MiniMax China)

La diferencia entre Anthropic y MiniMax es SOLO el base_url + api_key.
Por eso MiniMax hereda de esta clase y solo override el constructor.
"""
from __future__ import annotations

import logging

from anthropic import (
    APIError,
    APITimeoutError,
    Anthropic,
    AuthenticationError,
    RateLimitError,
)

from .provider import LlmProvider, LlmProviderError
from .types import ChatRequest, ChatResponse, ChatUsage

log = logging.getLogger(__name__)


class AnthropicCompatibleProvider(LlmProvider):
    """Provider base para APIs compatibles con Anthropic Messages API."""

    def __init__(self, api_key: str, base_url: str, model: str) -> None:
        if not api_key:
            raise LlmProviderError("api_key es requerido")
        if not base_url:
            raise LlmProviderError("base_url es requerido")
        if not model:
            raise LlmProviderError("model es requerido")

        self._client = Anthropic(api_key=api_key, base_url=base_url)
        self._model = model

    def chat_complete(self, request: ChatRequest) -> ChatResponse:
        messages_payload = [
            {"role": msg.role, "content": msg.content}
            for msg in request.messages
        ]

        kwargs: dict = {
            "model": request.model or self._model,
            "messages": messages_payload,
            "max_tokens": request.max_tokens,
            "temperature": request.temperature,
        }
        if request.system:
            kwargs["system"] = request.system

        try:
            response = self._client.messages.create(**kwargs)
        except AuthenticationError as e:
            raise LlmProviderError(f"auth invalida: {e}") from e
        except RateLimitError as e:
            raise LlmProviderError(f"rate limit: {e}") from e
        except APITimeoutError as e:
            raise LlmProviderError(f"timeout: {e}") from e
        except APIError as e:
            raise LlmProviderError(f"api error: {e}") from e

        content_text = "".join(
            block.text for block in response.content if getattr(block, "type", None) == "text"
        )

        usage = ChatUsage(
            input_tokens=getattr(response.usage, "input_tokens", 0),
            output_tokens=getattr(response.usage, "output_tokens", 0),
        )

        return ChatResponse(
            content=content_text,
            model=response.model,
            usage=usage,
            stop_reason=getattr(response, "stop_reason", "end_turn") or "end_turn",
        )


class AnthropicProvider(AnthropicCompatibleProvider):
    """Provider Anthropic oficial (api.anthropic.com)."""

    DEFAULT_BASE_URL = "https://api.anthropic.com"
    DEFAULT_MODEL = "claude-haiku-4-5"

    def __init__(self, api_key: str, model: str | None = None) -> None:
        super().__init__(
            api_key=api_key,
            base_url=self.DEFAULT_BASE_URL,
            model=model or self.DEFAULT_MODEL,
        )