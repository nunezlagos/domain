"""Tests del factory LlmFactory (HU-49.1).

Cubre la resolucion de provider segun env vars y los errores de config.
"""
from __future__ import annotations

import os
from unittest.mock import patch

import pytest

from core.llm import LlmProviderError
from core.llm.anthropic_provider import AnthropicProvider
from core.llm.factory import (
    LlmFactory,
    PROVIDER_ANTHROPIC,
    PROVIDER_DEFAULT,
    PROVIDER_MINIMAX,
)
from core.llm.minimax_provider import MinimaxProvider


@pytest.fixture(autouse=True)
def _clear_lru_cache():
    from core.llm import factory as factory_module
    factory_module.get_default_provider.cache_clear()
    yield
    factory_module.get_default_provider.cache_clear()


def test_make_minimax_con_env(monkeypatch):
    monkeypatch.setenv("LLM_PROVIDER", PROVIDER_MINIMAX)
    monkeypatch.setenv("MINIMAX_API_KEY", "sk-cp-test")
    monkeypatch.setenv("MINIMAX_MODEL", "MiniMax-M3")
    monkeypatch.setenv("MINIMAX_REGION", "international")

    provider = LlmFactory.make()

    assert isinstance(provider, MinimaxProvider)


def test_make_minimax_sin_key_falla(monkeypatch):
    monkeypatch.setenv("LLM_PROVIDER", PROVIDER_MINIMAX)
    monkeypatch.delenv("MINIMAX_API_KEY", raising=False)

    with pytest.raises(LlmProviderError, match="MINIMAX_API_KEY"):
        LlmFactory.make()


def test_make_anthropic_con_env(monkeypatch):
    monkeypatch.setenv("LLM_PROVIDER", PROVIDER_ANTHROPIC)
    monkeypatch.setenv("ANTHROPIC_API_KEY", "sk-ant-test")
    monkeypatch.setenv("ANTHROPIC_MODEL", "claude-haiku-4-5")

    provider = LlmFactory.make()

    assert isinstance(provider, AnthropicProvider)


def test_make_anthropic_sin_key_falla(monkeypatch):
    monkeypatch.setenv("LLM_PROVIDER", PROVIDER_ANTHROPIC)
    monkeypatch.delenv("ANTHROPIC_API_KEY", raising=False)

    with pytest.raises(LlmProviderError, match="ANTHROPIC_API_KEY"):
        LlmFactory.make()


def test_make_provider_desconocido_falla(monkeypatch):
    monkeypatch.setenv("LLM_PROVIDER", "openai")

    with pytest.raises(LlmProviderError, match="no soportado"):
        LlmFactory.make()


def test_make_default_sin_env_es_minimax(monkeypatch):
    monkeypatch.delenv("LLM_PROVIDER", raising=False)
    monkeypatch.setenv("MINIMAX_API_KEY", "sk-cp-test")

    provider = LlmFactory.make()

    assert isinstance(provider, MinimaxProvider)
    assert PROVIDER_DEFAULT == PROVIDER_MINIMAX


def test_make_con_base_url_custom(monkeypatch):
    monkeypatch.setenv("LLM_PROVIDER", PROVIDER_MINIMAX)
    monkeypatch.setenv("MINIMAX_API_KEY", "sk-cp-test")
    monkeypatch.setenv("MINIMAX_BASE_URL", "https://mi-proxy.example.com/anthropic")
    monkeypatch.setenv("MINIMAX_MODEL", "custom-model")

    provider = LlmFactory.make()

    assert provider._client.base_url.host == "mi-proxy.example.com"
    assert provider._model == "custom-model"


def test_make_minimax_region_china(monkeypatch):
    monkeypatch.setenv("LLM_PROVIDER", PROVIDER_MINIMAX)
    monkeypatch.setenv("MINIMAX_API_KEY", "sk-cp-test")
    monkeypatch.setenv("MINIMAX_REGION", "china")

    provider = LlmFactory.make()

    assert "minimaxi.com" in str(provider._client.base_url)