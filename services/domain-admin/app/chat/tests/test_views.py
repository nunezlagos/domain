"""Tests de los views del chat (HU-49.2)."""
from __future__ import annotations

import json
from datetime import datetime, timezone
from unittest.mock import MagicMock, patch

import pytest
from django.test import RequestFactory

from chat import views as chat_views


@pytest.fixture
def factory():
    return RequestFactory()


@pytest.fixture
def authed_request(factory):
    request = factory.get("/chat/api/conversations")
    request.session = {"authenticated": True, "email": "admin@admin.com"}
    return request


def _conv_mock(conv_id="c-1", title="Test", last_msg_preview=""):
    conv = MagicMock()
    conv.id = conv_id
    conv.title = title
    conv.created_at = datetime(2026, 6, 26, tzinfo=timezone.utc)
    conv.updated_at = datetime(2026, 6, 26, tzinfo=timezone.utc)
    last = MagicMock()
    last.content = last_msg_preview
    conv.messages.order_by.return_value.first.return_value = last if last_msg_preview else None
    return conv


def test_list_conversations_sin_auth_redirige(factory):
    request = factory.get("/chat/api/conversations")
    request.session = {}
    response = chat_views.list_conversations(request)
    assert response.status_code == 302
    assert "/login/" in response.url


@patch("chat.views._get_service")
def test_list_conversations_autenticado_devuelve_json(mock_get_service, authed_request):
    svc = MagicMock()
    svc.list_conversations.return_value = [_conv_mock("c-1", "Hola", "ultimo mensaje")]
    mock_get_service.return_value = svc

    response = chat_views.list_conversations(authed_request)
    assert response.status_code == 200
    data = json.loads(response.content)
    assert "data" in data
    assert len(data["data"]) == 1
    assert data["data"][0]["title"] == "Hola"
    assert data["data"][0]["last_message_preview"] == "ultimo mensaje"


@patch("chat.views._get_service")
def test_create_conversation_sin_titulo(mock_get_service, factory):
    request = factory.post(
        "/chat/api/conversations/new",
        data="{}",
        content_type="application/json",
    )
    request.session = {"authenticated": True, "email": "admin@admin.com"}
    svc = MagicMock()
    svc.create_conversation.return_value = _conv_mock("c-new")
    mock_get_service.return_value = svc

    response = chat_views.create_conversation(request)
    assert response.status_code == 201
    data = json.loads(response.content)
    assert data["data"]["id"] == "c-new"
    svc.create_conversation.assert_called_once_with("admin@admin.com", title="")


@patch("chat.views._get_service")
def test_create_conversation_con_titulo(mock_get_service, factory):
    request = factory.post(
        "/chat/api/conversations/new",
        data='{"title": "Mi chat"}',
        content_type="application/json",
    )
    request.session = {"authenticated": True, "email": "admin@admin.com"}
    svc = MagicMock()
    svc.create_conversation.return_value = _conv_mock("c-new", "Mi chat")
    mock_get_service.return_value = svc

    response = chat_views.create_conversation(request)
    assert response.status_code == 201
    svc.create_conversation.assert_called_once_with("admin@admin.com", title="Mi chat")


@patch("chat.views._get_service")
def test_create_message_requiere_content(mock_get_service, factory):
    request = factory.post(
        "/chat/api/conversations/c-1/messages/new",
        data="{}",
        content_type="application/json",
    )
    request.session = {"authenticated": True, "email": "admin@admin.com"}

    response = chat_views.create_message(request, "c-1")
    assert response.status_code == 400


@patch("chat.views._get_service")
def test_create_message_sin_auth_redirige(mock_get_service, factory):
    request = factory.post(
        "/chat/api/conversations/c-1/messages/new",
        data='{"content":"hola"}',
        content_type="application/json",
    )
    request.session = {}
    response = chat_views.create_message(request, "c-1")
    assert response.status_code == 302


@patch("chat.views._get_service")
def test_create_message_no_puede_acceder_a_otra_conv(mock_get_service, factory):
    request = factory.post(
        "/chat/api/conversations/c-1/messages/new",
        data='{"content":"hola"}',
        content_type="application/json",
    )
    request.session = {"authenticated": True, "email": "otro@admin.com"}
    svc = MagicMock()
    svc._get_owned.side_effect = PermissionError("no encontrada")
    mock_get_service.return_value = svc

    response = chat_views.create_message(request, "c-1")
    assert response.status_code == 403


@patch("chat.views._get_service")
def test_create_message_exitosa_retorna_202(mock_get_service, factory):
    request = factory.post(
        "/chat/api/conversations/c-1/messages/new",
        data='{"content":"hola"}',
        content_type="application/json",
    )
    request.session = {"authenticated": True, "email": "admin@admin.com"}
    svc = MagicMock()
    svc._get_owned.return_value = _conv_mock("c-1")
    assistant = MagicMock()
    assistant.id = 99
    assistant.conversation_id = "c-1"
    assistant.role = "assistant"
    assistant.content = "Hola, soy la respuesta"
    assistant.content_partial = None
    assistant.status = "completed"
    assistant.sources = []
    assistant.tokens_in = 10
    assistant.tokens_out = 5
    assistant.model = "MiniMax-M3"
    assistant.duration_ms = 100
    assistant.error_message = None
    assistant.created_at = datetime(2026, 6, 26, tzinfo=timezone.utc)
    svc.create_messages.return_value = assistant
    mock_get_service.return_value = svc

    response = chat_views.create_message(request, "c-1")
    assert response.status_code == 202
    data = json.loads(response.content)
    assert data["data"]["id"] == 99
    assert data["data"]["status"] == "completed"
    assert data["data"]["content"] == "Hola, soy la respuesta"
    svc.process_message.assert_called_once_with(assistant, "hola")


@patch("chat.views._get_service")
def test_delete_conversation_exitoso(mock_get_service, factory):
    request = factory.delete("/chat/api/conversations/c-1")
    request.session = {"authenticated": True, "email": "admin@admin.com"}
    svc = MagicMock()
    mock_get_service.return_value = svc

    response = chat_views.delete_conversation(request, "c-1")
    assert response.status_code == 200
    svc.delete_conversation.assert_called_once_with("c-1", "admin@admin.com")


@patch("chat.views._get_service")
def test_delete_conversation_forbidden(mock_get_service, factory):
    request = factory.delete("/chat/api/conversations/c-1")
    request.session = {"authenticated": True, "email": "otro@admin.com"}
    svc = MagicMock()
    svc.delete_conversation.side_effect = PermissionError("no encontrada")
    mock_get_service.return_value = svc

    response = chat_views.delete_conversation(request, "c-1")
    assert response.status_code == 403


@patch("chat.admin_models.Message.objects")
def test_show_message_existe(mock_msg, factory):
    request = factory.get("/chat/api/messages/42")
    request.session = {"authenticated": True, "email": "admin@admin.com"}
    msg = MagicMock()
    msg.id = 42
    msg.conversation_id = "c-1"
    msg.role = "assistant"
    msg.content = "respuesta"
    msg.content_partial = None
    msg.status = "completed"
    msg.sources = []
    msg.tokens_in = 0
    msg.tokens_out = 0
    msg.model = ""
    msg.duration_ms = 0
    msg.error_message = None
    msg.created_at = datetime(2026, 6, 26, tzinfo=timezone.utc)
    conv = MagicMock()
    conv.user_email = "admin@admin.com"
    conv.deleted_at = None
    msg.conversation = conv
    mock_msg.get.return_value = msg

    response = chat_views.show_message(request, 42)
    assert response.status_code == 200
    assert json.loads(response.content)["data"]["content"] == "respuesta"


@patch("chat.admin_models.Message.objects")
def test_show_message_no_existe_retorna_404(mock_msg, factory):
    from chat.admin_models import Message as Msg
    request = factory.get("/chat/api/messages/999")
    request.session = {"authenticated": True, "email": "admin@admin.com"}
    mock_msg.get.side_effect = Msg.DoesNotExist()

    response = chat_views.show_message(request, 999)
    assert response.status_code == 404


@patch("chat.admin_models.Message.objects")
def test_show_message_de_otro_usuario_forbidden(mock_msg, factory):
    request = factory.get("/chat/api/messages/42")
    request.session = {"authenticated": True, "email": "otro@admin.com"}
    msg = MagicMock()
    conv = MagicMock()
    conv.user_email = "admin@admin.com"
    conv.deleted_at = None
    msg.conversation = conv
    mock_msg.get.return_value = msg

    response = chat_views.show_message(request, 42)
    assert response.status_code == 403


@patch("chat.views._get_service")
def test_list_messages_ok(mock_get_service, factory):
    request = factory.get("/chat/api/conversations/c-1/messages")
    request.session = {"authenticated": True, "email": "admin@admin.com"}
    svc = MagicMock()
    svc.list_messages.return_value = []
    mock_get_service.return_value = svc

    response = chat_views.list_messages(request, "c-1")
    assert response.status_code == 200
    assert json.loads(response.content)["data"] == []


@patch("chat.views._get_service")
def test_list_messages_forbidden(mock_get_service, factory):
    request = factory.get("/chat/api/conversations/c-1/messages")
    request.session = {"authenticated": True, "email": "otro@admin.com"}
    svc = MagicMock()
    svc.list_messages.side_effect = PermissionError("no encontrada")
    mock_get_service.return_value = svc

    response = chat_views.list_messages(request, "c-1")
    assert response.status_code == 403


def test_parse_body_json_invalido(factory):
    request = factory.post("/x", data="not json", content_type="application/json")
    assert chat_views._parse_body(request) is None


def test_parse_body_vacio(factory):
    request = factory.post("/x", data="", content_type="application/json")
    assert chat_views._parse_body(request) is None


def test_parse_body_json_valido(factory):
    request = factory.post(
        "/x", data='{"a": 1}', content_type="application/json"
    )
    assert chat_views._parse_body(request) == {"a": 1}