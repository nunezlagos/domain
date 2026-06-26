"""HU-49.2: views del chat IA (endpoints REST + pagina HTML).

Endpoints REST (JSON):
- GET  /chat/api/conversations
- POST /chat/api/conversations
- GET  /chat/api/conversations/<uuid>/messages
- POST /chat/api/conversations/<uuid>/messages
- GET  /chat/api/messages/<id>
- DELETE /chat/api/conversations/<uuid>

Pagina HTML (HU-49.3 la implementa, pero el view ya existe):
- GET  /chat/

Auth: mismo patron que el resto del admin (HU-47.1 single-user). El
"user_email" se toma de `request.session["email"]`. Para multi-user
futuro, cambiar por `request.user.email`.

Seguridad: rate limit por usuario (10 req/min) implementado en
`rate_limit.py` (defense contra LLM04 Model DoS).
"""
from __future__ import annotations

import json
import logging
from dataclasses import asdict

from django.http import HttpRequest, HttpResponse, JsonResponse
from django.shortcuts import render
from django.urls import reverse
from django.views.decorators.csrf import csrf_exempt
from django.views.decorators.http import require_http_methods

from core.auth import require_auth
from core.llm import LlmFactory, LlmProviderError

from .admin_models import Conversation, Message
from .rate_limit import check_default
from .retrieval import RetrievalService
from .services import ChatService

log = logging.getLogger(__name__)


def _serialize_conversation(c: Conversation) -> dict:
    last_msg = c.messages.order_by("-created_at").first() if c.id else None
    return {
        "id": str(c.id),
        "title": c.title,
        "created_at": c.created_at.isoformat() if c.created_at else None,
        "updated_at": c.updated_at.isoformat() if c.updated_at else None,
        "last_message_preview": (last_msg.content[:80] if last_msg and last_msg.content else ""),
    }


def _serialize_message(m: Message) -> dict:
    return {
        "id": m.id,
        "conversation_id": str(m.conversation_id),
        "role": m.role,
        "content": m.content,
        "content_partial": m.content_partial,
        "status": m.status,
        "sources": m.sources or [],
        "tokens_in": m.tokens_in,
        "tokens_out": m.tokens_out,
        "model": m.model,
        "duration_ms": m.duration_ms,
        "error_message": m.error_message,
        "created_at": m.created_at.isoformat() if m.created_at else None,
    }


def _get_service() -> ChatService:
    """Factory del service con dependencias reales.

    En tests se mockea via `chat.services.ChatService` directo.
    """
    retrieval = RetrievalService()
    llm = LlmFactory.make()
    return ChatService(retrieval=retrieval, llm=llm)


def _current_email(request: HttpRequest) -> str:
    return request.session.get("email", "anonymous@local")


@require_http_methods(["GET"])
def chat_page(request: HttpRequest) -> HttpResponse:
    redir = require_auth(request)
    if redir:
        return redir
    return render(request, "dashboard.html", {})


@require_http_methods(["GET"])
def list_conversations(request: HttpRequest) -> JsonResponse:
    redir = require_auth(request)
    if redir:
        return redir
    service = _get_service()
    convs = service.list_conversations(_current_email(request))
    return JsonResponse({"data": [_serialize_conversation(c) for c in convs]})


@csrf_exempt
@require_http_methods(["POST"])
def create_conversation(request: HttpRequest) -> JsonResponse:
    redir = require_auth(request)
    if redir:
        return redir
    body = _parse_body(request)
    title = (body or {}).get("title", "").strip()
    service = _get_service()
    conv = service.create_conversation(_current_email(request), title=title)
    return JsonResponse({"data": _serialize_conversation(conv)}, status=201)


@require_http_methods(["GET"])
def list_messages(request: HttpRequest, conversation_id: str) -> JsonResponse:
    redir = require_auth(request)
    if redir:
        return redir
    service = _get_service()
    try:
        msgs = service.list_messages(conversation_id, _current_email(request))
    except PermissionError:
        return JsonResponse({"error": "forbidden"}, status=403)
    return JsonResponse({"data": [_serialize_message(m) for m in msgs]})


@csrf_exempt
@require_http_methods(["POST"])
def create_message(request: HttpRequest, conversation_id: str) -> JsonResponse:
    redir = require_auth(request)
    if redir:
        return redir
    email = _current_email(request)
    rl = check_default(email)
    if not rl.allowed:
        return JsonResponse(
            {
                "error": "rate limit exceeded",
                "message": f"demasiadas requests. intenta en {rl.reset_in_seconds}s",
                "reset_in": rl.reset_in_seconds,
            },
            status=429,
        )

    body = _parse_body(request)
    if not body or "content" not in body:
        return JsonResponse({"error": "content requerido"}, status=400)
    content = body["content"].strip()
    if not content:
        return JsonResponse({"error": "content vacio"}, status=400)

    service = _get_service()
    try:
        conv = service._get_owned(conversation_id, email)
    except PermissionError:
        return JsonResponse({"error": "forbidden"}, status=403)

    assistant = service.create_messages(conv, content)
    try:
        service.process_message(assistant, content)
    except LlmProviderError as e:
        log.exception("chat: llm error en create_message")
    return JsonResponse({"data": _serialize_message(assistant)}, status=202)


@require_http_methods(["GET"])
def show_message(request: HttpRequest, message_id: int) -> JsonResponse:
    redir = require_auth(request)
    if redir:
        return redir
    try:
        msg = Message.objects.get(id=message_id)
    except Message.DoesNotExist:
        return JsonResponse({"error": "not found"}, status=404)
    email = _current_email(request)
    if msg.conversation.user_email != email or msg.conversation.deleted_at is not None:
        return JsonResponse({"error": "forbidden"}, status=403)
    return JsonResponse({"data": _serialize_message(msg)})


@csrf_exempt
@require_http_methods(["DELETE"])
def delete_conversation(request: HttpRequest, conversation_id: str) -> JsonResponse:
    redir = require_auth(request)
    if redir:
        return redir
    service = _get_service()
    try:
        service.delete_conversation(conversation_id, _current_email(request))
    except PermissionError:
        return JsonResponse({"error": "forbidden"}, status=403)
    return JsonResponse({"data": {"deleted": True}})


def _parse_body(request: HttpRequest) -> dict | None:
    if not request.body:
        return None
    try:
        return json.loads(request.body)
    except json.JSONDecodeError:
        return None