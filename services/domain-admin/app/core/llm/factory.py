"""HU-49.1: factory que resuelve el provider segun env vars.

La seleccion se hace por LLM_PROVIDER (default: minimax). Cada provider
tiene su propio set de env vars. La factory NO instancia el provider en
import time (lazy) para no romper el arranque del admin cuando falta
configuracion (ej: tests unitarios, comandos de manage.py sin LLM).
"""
from __future__ import annotations

import logging
import os
from functools import lru_cache

from .anthropic_provider import AnthropicProvider
from .minimax_provider import MinimaxProvider
from .provider import LlmProvider, LlmProviderError

log = logging.getLogger(__name__)

PROVIDER_MINIMAX = "minimax"
PROVIDER_ANTHROPIC = "anthropic"
PROVIDER_DEFAULT = PROVIDER_MINIMAX


class LlmFactory:
    """Factory que resuelve el provider desde env vars."""

    @staticmethod
    def make(provider_name: str | None = None) -> LlmProvider:
        name = (provider_name or os.environ.get("LLM_PROVIDER") or PROVIDER_DEFAULT).lower()

        if name == PROVIDER_MINIMAX:
            return LlmFactory._make_minimax()
        if name == PROVIDER_ANTHROPIC:
            return LlmFactory._make_anthropic()

        raise LlmProviderError(f"LLM provider no soportado: {name}")

    @staticmethod
    def _make_minimax() -> MinimaxProvider:
        api_key = os.environ.get("MINIMAX_API_KEY", "")
        if not api_key:
            raise LlmProviderError(
                "MINIMAX_API_KEY no configurada. "
                "Configurala en .env o cambia LLM_PROVIDER=anthropic."
            )
        model = os.environ.get("MINIMAX_MODEL") or MinimaxProvider.DEFAULT_MODEL
        region = os.environ.get("MINIMAX_REGION", "international")
        base_url = os.environ.get("MINIMAX_BASE_URL")
        log.info("llm.provider: minimax model=%s region=%s", model, region)
        if base_url:
            from .anthropic_provider import AnthropicCompatibleProvider
            return AnthropicCompatibleProvider(
                api_key=api_key, base_url=base_url, model=model
            )
        return MinimaxProvider(api_key=api_key, model=model, region=region)

    @staticmethod
    def _make_anthropic() -> AnthropicProvider:
        api_key = os.environ.get("ANTHROPIC_API_KEY", "")
        if not api_key:
            raise LlmProviderError("ANTHROPIC_API_KEY no configurada")
        model = os.environ.get("ANTHROPIC_MODEL") or AnthropicProvider.DEFAULT_MODEL
        log.info("llm.provider: anthropic model=%s", model)
        return AnthropicProvider(api_key=api_key, model=model)


@lru_cache(maxsize=1)
def get_default_provider() -> LlmProvider:
    """Provider por default cacheado para uso trivial en views.

    Si la config falla (key faltante) el error se levanta aca. Para casos
    que requieren lazy resolution (background jobs, tests), usar
    LlmFactory.make() directo.
    """
    return LlmFactory.make()