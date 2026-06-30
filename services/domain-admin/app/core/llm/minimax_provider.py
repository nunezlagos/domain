"""HU-49.1: provider MiniMax (Anthropic-compatible).

MiniMax expone un endpoint Anthropic-compatible en
https://api.minimax.io/anthropic (internacional) o
https://api.minimaxi.com/anthropic (China). Hereda de
AnthropicCompatibleProvider y solo configura base_url + model + api_key
desde env vars.

Endpoint y formato validados con curl contra el token del usuario
(MiniMax-M3 con context window 1M tokens).
"""
from __future__ import annotations

from .anthropic_provider import AnthropicCompatibleProvider


class MinimaxProvider(AnthropicCompatibleProvider):
    """Provider MiniMax (Anthropic-compatible)."""

    INTERNATIONAL_BASE_URL = "https://api.minimax.io/anthropic"
    CHINA_BASE_URL = "https://api.minimaxi.com/anthropic"
    DEFAULT_MODEL = "MiniMax-M3"

    def __init__(
        self,
        api_key: str,
        model: str | None = None,
        region: str = "international",
    ) -> None:
        base_url = (
            self.INTERNATIONAL_BASE_URL if region == "international" else self.CHINA_BASE_URL
        )
        super().__init__(
            api_key=api_key,
            base_url=base_url,
            model=model or self.DEFAULT_MODEL,
        )
        self._region = region