"""Tests del MinimaxProvider (HU-49.1).

Mockeamos la SDK Anthropic para no pegar contra la API real (rate limit
alto de MiniMax). Solo validamos: configuracion del cliente, envio de
mensajes, parseo de respuesta, manejo de errores.
"""
from __future__ import annotations

from unittest.mock import MagicMock, patch

import pytest

from core.llm import LlmProviderError
from core.llm.minimax_provider import MinimaxProvider
from core.llm.types import ChatMessage, ChatRequest


@pytest.fixture
def mock_anthropic_messages():
    with patch("core.llm.anthropic_provider.Anthropic") as mock_cls:
        mock_client = MagicMock()
        mock_cls.return_value = mock_client
        yield mock_client.messages


def _text_block(text: str) -> MagicMock:
    block = MagicMock()
    block.type = "text"
    block.text = text
    return block


def _messages_response(text: str, model: str = "MiniMax-M3", in_t: int = 10, out_t: int = 5):
    resp = MagicMock()
    resp.content = [_text_block(text)]
    resp.model = model
    resp.usage.input_tokens = in_t
    resp.usage.output_tokens = out_t
    resp.stop_reason = "end_turn"
    return resp


def test_minimax_construye_cliente_con_base_url_internacional():
    provider = MinimaxProvider(api_key="sk-cp-test", region="international")
    assert "api.minimax.io" in str(provider._client.base_url)


def test_minimax_construye_cliente_con_base_url_china():
    provider = MinimaxProvider(api_key="sk-cp-test", region="china")
    assert "api.minimaxi.com" in str(provider._client.base_url)


def test_minimax_model_default():
    provider = MinimaxProvider(api_key="sk-cp-test")
    assert provider._model == "MiniMax-M3"


def test_chat_complete_devuelve_contenido(mock_anthropic_messages):
    mock_anthropic_messages.create.return_value = _messages_response(
        "Hola, soy MiniMax-M3", in_t=42, out_t=8
    )
    provider = MinimaxProvider(api_key="sk-cp-test")

    req = ChatRequest(
        messages=[ChatMessage(role="user", content="Hola")],
        system="Eres un asistente",
        max_tokens=100,
    )
    resp = provider.chat_complete(req)

    assert resp.content == "Hola, soy MiniMax-M3"
    assert resp.model == "MiniMax-M3"
    assert resp.usage.input_tokens == 42
    assert resp.usage.output_tokens == 8


def test_chat_complete_envia_system_y_messages(mock_anthropic_messages):
    mock_anthropic_messages.create.return_value = _messages_response("ok")
    provider = MinimaxProvider(api_key="sk-cp-test")

    req = ChatRequest(
        messages=[
            ChatMessage(role="user", content="primera"),
            ChatMessage(role="assistant", content="primera respuesta"),
            ChatMessage(role="user", content="segunda"),
        ],
        system="sos un tester",
    )
    provider.chat_complete(req)

    call_kwargs = mock_anthropic_messages.create.call_args.kwargs
    assert call_kwargs["system"] == "sos un tester"
    assert call_kwargs["messages"] == [
        {"role": "user", "content": "primera"},
        {"role": "assistant", "content": "primera respuesta"},
        {"role": "user", "content": "segunda"},
    ]
    assert call_kwargs["model"] == "MiniMax-M3"


def test_chat_complete_omite_system_si_vacio(mock_anthropic_messages):
    mock_anthropic_messages.create.return_value = _messages_response("ok")
    provider = MinimaxProvider(api_key="sk-cp-test")

    req = ChatRequest(messages=[ChatMessage(role="user", content="ping")])
    provider.chat_complete(req)

    call_kwargs = mock_anthropic_messages.create.call_args.kwargs
    assert "system" not in call_kwargs


def test_chat_complete_request_model_override(mock_anthropic_messages):
    mock_anthropic_messages.create.return_value = _messages_response("ok")
    provider = MinimaxProvider(api_key="sk-cp-test")

    req = ChatRequest(
        messages=[ChatMessage(role="user", content="ping")],
        model="custom-model",
    )
    provider.chat_complete(req)

    assert mock_anthropic_messages.create.call_args.kwargs["model"] == "custom-model"


def test_chat_complete_traduce_auth_error(mock_anthropic_messages):
    from anthropic import AuthenticationError

    err = AuthenticationError(
        message="invalid api key", response=MagicMock(status_code=401), body=None
    )
    mock_anthropic_messages.create.side_effect = err
    provider = MinimaxProvider(api_key="sk-cp-bad")

    with pytest.raises(LlmProviderError, match="auth invalida"):
        provider.chat_complete(
            ChatRequest(messages=[ChatMessage(role="user", content="x")])
        )


def test_chat_complete_traduce_rate_limit(mock_anthropic_messages):
    from anthropic import RateLimitError

    err = RateLimitError(
        message="rate exceeded", response=MagicMock(status_code=429), body=None
    )
    mock_anthropic_messages.create.side_effect = err
    provider = MinimaxProvider(api_key="sk-cp-test")

    with pytest.raises(LlmProviderError, match="rate limit"):
        provider.chat_complete(
            ChatRequest(messages=[ChatMessage(role="user", content="x")])
        )


def test_chat_complete_traduce_timeout(mock_anthropic_messages):
    from anthropic import APITimeoutError

    mock_anthropic_messages.create.side_effect = APITimeoutError(request=MagicMock())
    provider = MinimaxProvider(api_key="sk-cp-test")

    with pytest.raises(LlmProviderError, match="timeout"):
        provider.chat_complete(
            ChatRequest(messages=[ChatMessage(role="user", content="x")])
        )


def test_constructor_valida_args():
    from core.llm.anthropic_provider import AnthropicCompatibleProvider

    with pytest.raises(LlmProviderError, match="api_key"):
        MinimaxProvider(api_key="")
    with pytest.raises(LlmProviderError, match="model"):
        AnthropicCompatibleProvider(
            api_key="sk-cp-test", base_url="https://x", model=""
        )


def test_response_con_bloques_no_text(mock_anthropic_messages):
    resp = MagicMock()
    text_block = _text_block("respuesta limpia")
    other_block = MagicMock()
    other_block.type = "tool_use"
    resp.content = [other_block, text_block]
    resp.model = "MiniMax-M3"
    resp.usage.input_tokens = 0
    resp.usage.output_tokens = 0
    resp.stop_reason = "end_turn"
    mock_anthropic_messages.create.return_value = resp

    provider = MinimaxProvider(api_key="sk-cp-test")
    out = provider.chat_complete(
        ChatRequest(messages=[ChatMessage(role="user", content="x")])
    )

    assert out.content == "respuesta limpia"