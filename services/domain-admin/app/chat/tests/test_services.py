"""Tests del ChatService (HU-49.2)."""
from __future__ import annotations

from datetime import datetime, timezone
from unittest.mock import MagicMock

import pytest

from chat.admin_models import Conversation, Message
from chat.models import RagContext, Source
from chat.services import ChatService, _title_from_question


def _make_conv(**kwargs):
    defaults = dict(
        id="conv-uuid-1234",
        user_email="admin@admin.com",
        title="",
        created_at=datetime(2026, 6, 26, tzinfo=timezone.utc),
        updated_at=datetime(2026, 6, 26, tzinfo=timezone.utc),
        deleted_at=None,
    )
    defaults.update(kwargs)
    conv = MagicMock()
    for k, v in defaults.items():
        setattr(conv, k, v)
    return conv


def _make_message(**kwargs):
    defaults = dict(
        id=42,
        conversation_id="conv-uuid-1234",
        role=Message.ROLE_ASSISTANT,
        content=None,
        content_partial=None,
        status=Message.STATUS_PENDING,
        sources=[],
        tokens_in=0,
        tokens_out=0,
        model="",
        duration_ms=0,
        error_message=None,
        created_at=datetime(2026, 6, 26, tzinfo=timezone.utc),
    )
    defaults.update(kwargs)
    msg = MagicMock()
    for k, v in defaults.items():
        setattr(msg, k, v)
    return msg


def _make_retrieval(context: RagContext) -> MagicMock:
    r = MagicMock()
    r.retrieve.return_value = context
    return r


def _make_llm(content: str = "Respuesta del LLM", in_t: int = 10, out_t: int = 5) -> MagicMock:
    from core.llm import ChatResponse, ChatUsage
    llm = MagicMock()
    llm.chat_complete.return_value = ChatResponse(
        content=content,
        model="MiniMax-M3",
        usage=ChatUsage(input_tokens=in_t, output_tokens=out_t),
    )
    return llm


def test_title_from_question_truncates():
    assert _title_from_question("hola mundo") == "hola mundo"
    long = "palabra " * 30
    title = _title_from_question(long)
    assert title.endswith("...")


def test_create_messages_crea_user_y_assistant(monkeypatch):
    monkeypatch.setattr(
        "chat.services.Message.objects.create",
        lambda **kw: _make_message(role=kw.get("role"), status=kw.get("status")),
    )
    service = ChatService(retrieval=MagicMock(), llm=MagicMock())
    conv = _make_conv()
    assistant = service.create_messages(conv, "que agentes hay?")

    assert assistant.role == Message.ROLE_ASSISTANT
    assert assistant.status == Message.STATUS_PENDING


def test_create_messages_setea_titulo_si_primera_vez(monkeypatch):
    captured = {}

    def fake_create(**kw):
        captured.setdefault(kw.get("role"), []).append(kw)
        return _make_message(role=kw.get("role"), status=kw.get("status"))

    monkeypatch.setattr("chat.services.Message.objects.create", fake_create)
    service = ChatService(retrieval=MagicMock(), llm=MagicMock())
    conv = _make_conv(title="")
    service.create_messages(conv, "cuantos skills activos hay?")

    assert conv.title == "cuantos skills activos hay?"


def test_create_messages_no_pisa_titulo_si_ya_existe(monkeypatch):
    monkeypatch.setattr(
        "chat.services.Message.objects.create",
        lambda **kw: _make_message(role=kw.get("role"), status=kw.get("status")),
    )
    service = ChatService(retrieval=MagicMock(), llm=MagicMock())
    conv = _make_conv(title="titulo previo")
    service.create_messages(conv, "segunda pregunta")
    assert conv.title == "titulo previo"


def test_process_message_sin_contexto_responde_fallback():
    retrieval = _make_retrieval(RagContext(is_empty=True))
    llm = MagicMock()
    service = ChatService(retrieval=retrieval, llm=llm)
    msg = _make_message()
    service.process_message(msg, "que agentes hay?")

    assert msg.status == Message.STATUS_COMPLETED
    assert "No encontre" in msg.content
    assert llm.chat_complete.call_count == 0


def test_process_message_con_contexto_llama_llm():
    ctx = RagContext(
        chunks=["Agent: Bot | description=test"],
        sources=[Source(table="agent", id="1", title="Bot", snippet="test", score=0.85)],
        is_empty=False,
    )
    retrieval = _make_retrieval(ctx)
    llm = _make_llm(content="Hay un agente llamado **Bot**", in_t=20, out_t=8)
    service = ChatService(retrieval=retrieval, llm=llm)
    service._build_history = lambda *args, **kwargs: []
    msg = _make_message()
    service.process_message(msg, "que agentes hay?")

    assert msg.status == Message.STATUS_COMPLETED
    assert msg.content == "Hay un agente llamado **Bot**"
    assert msg.tokens_in == 20
    assert msg.tokens_out == 8
    assert msg.model == "MiniMax-M3"
    assert len(msg.sources) == 1
    assert msg.sources[0]["titulo"] == "Bot"
    assert llm.chat_complete.call_count == 1


def test_process_message_llm_falla_marca_error():
    from core.llm import LlmProviderError

    ctx = RagContext(chunks=["x"], sources=[Source("agent", "1", "X", "x", 0.8)], is_empty=False)
    retrieval = _make_retrieval(ctx)
    llm = MagicMock()
    llm.chat_complete.side_effect = LlmProviderError("api timeout")
    service = ChatService(retrieval=retrieval, llm=llm)
    service._build_history = lambda *a, **k: []
    msg = _make_message()
    service.process_message(msg, "test")

    assert msg.status == Message.STATUS_ERROR
    assert "api timeout" in (msg.error_message or "")


def test_process_message_retrieval_falla_marca_error():
    retrieval = MagicMock()
    retrieval.retrieve.side_effect = RuntimeError("db caida")
    llm = MagicMock()
    service = ChatService(retrieval=retrieval, llm=llm)
    msg = _make_message()
    service.process_message(msg, "test")

    assert msg.status == Message.STATUS_ERROR
    assert llm.chat_complete.call_count == 0


def test_process_message_setea_processing_y_completed():
    statuses_seen = []

    ctx = RagContext(
        chunks=["x"],
        sources=[Source("agent", "1", "X", "x", 0.8)],
        is_empty=False,
    )
    retrieval = _make_retrieval(ctx)
    llm = _make_llm(content="ok")
    service = ChatService(retrieval=retrieval, llm=llm)
    service._build_history = lambda *a, **k: []
    msg = _make_message()

    original_save = msg.save
    def track_save(*args, **kwargs):
        statuses_seen.append(msg.status)
        original_save(*args, **kwargs)
    msg.save = track_save

    service.process_message(msg, "test")
    assert Message.STATUS_PROCESSING in statuses_seen
    assert statuses_seen[-1] == Message.STATUS_COMPLETED


def test_process_message_incluye_historial_en_llamada(monkeypatch):
    from core.llm import ChatMessage as LlmChatMessage
    ctx = RagContext(
        chunks=["Agent: Bot"],
        sources=[Source("agent", "1", "Bot", "Bot", 0.8)],
        is_empty=False,
    )
    retrieval = _make_retrieval(ctx)
    llm = _make_llm()
    service = ChatService(retrieval=retrieval, llm=llm)

    history_msgs = [
        LlmChatMessage(role="user", content="hola"),
        LlmChatMessage(role="assistant", content="hola!"),
    ]
    service._build_history = lambda *a, **k: history_msgs

    msg = _make_message()
    service.process_message(msg, "que hace?")

    req = llm.chat_complete.call_args.args[0]
    msgs = req.messages
    assert msgs[-1].role == "user"
    assert msgs[-1].content == "que hace?"
    assert msgs[-2].content == "hola!"


def test_process_message_sin_system_vacio_fallback_vacio():
    ctx = RagContext(
        chunks=[],
        sources=[],
        is_empty=True,
    )
    retrieval = _make_retrieval(ctx)
    llm = _make_llm()
    service = ChatService(retrieval=retrieval, llm=llm)
    msg = _make_message()
    service.process_message(msg, "test")
    assert msg.status == Message.STATUS_COMPLETED
    assert "No encontre" in msg.content
    assert llm.chat_complete.call_count == 0


def test_process_message_calcula_duracion():
    import time as time_mod

    ctx = RagContext(
        chunks=["x"],
        sources=[Source("agent", "1", "X", "x", 0.8)],
        is_empty=False,
    )
    retrieval = _make_retrieval(ctx)

    def slow_retrieve(_):
        time_mod.sleep(0.01)
        return ctx

    retrieval.retrieve.side_effect = slow_retrieve
    llm = _make_llm()
    service = ChatService(retrieval=retrieval, llm=llm)
    service._build_history = lambda *a, **k: []
    msg = _make_message()
    service.process_message(msg, "test")
    assert msg.duration_ms >= 10