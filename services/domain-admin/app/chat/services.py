"""HU-49.2: ChatService orquesta retrieval + LLM + persistencia.

Responsabilidades:
- `create_conversation(user_email)` -> Conversation nueva.
- `create_messages(conversation, question)` -> crea user msg + assistant
  placeholder, y dispara el procesamiento async (en este MVP: sync).
- `process_message(assistant_message, question)` -> RAG + LLM + persiste.
- `list_conversations(user_email)` -> query ordenado.
- `list_messages(conversation_id, user_email)` -> paginado.
- `delete_conversation(conversation_id, user_email)` -> soft delete.

Decisiones MVP:
- Procesamiento SINCRONO en el request (no hay Celery/queue todavia).
  Polling de 1.5s del front funciona igual. Cuando se sume SSE/queue
  en HU-49.4, se desacopla a async.
- Privacy guard: NUNCA expone api_key_ciphertext ni password_hash ni
  otros campos sensibles (el retrieval no los pide a la DB, asi que
  es defense-in-depth: ademas de no traerlos, no se confia en el
  LLM para filtrarlos).
"""
from __future__ import annotations

import logging
import time
from datetime import datetime, timezone

from django.utils.text import Truncator

from core.llm import ChatMessage as LlmChatMessage
from core.llm import ChatRequest
from core.llm import LlmProvider
from core.llm import LlmProviderError

from .admin_models import Conversation, Message
from .models import Source
from .prompts import build_system_prompt, build_user_message
from .retrieval import RetrievalService

log = logging.getLogger(__name__)

CONTEXT_WINDOW = 10


def _title_from_question(question: str) -> str:
    """Trunca una pregunta a 60 chars para usar como titulo inicial."""
    q = question.strip().replace("\n", " ")
    if len(q) <= 60:
        return q
    truncator = Truncator(q)
    return truncator.words(8, truncate="...")


def _now() -> datetime:
    return datetime.now(tz=timezone.utc)


class ChatService:
    """Orquesta el chat. Inyectable: tests pasan mocks del LlmProvider."""

    def __init__(
        self,
        retrieval: RetrievalService,
        llm: LlmProvider,
    ) -> None:
        self._retrieval = retrieval
        self._llm = llm

    def list_conversations(self, user_email: str) -> list[Conversation]:
        return list(
            Conversation.objects.filter(
                user_email=user_email, deleted_at__isnull=True
            ).order_by("-updated_at")
        )

    def create_conversation(self, user_email: str, title: str = "") -> Conversation:
        return Conversation.objects.create(
            user_email=user_email,
            title=title,
        )

    def list_messages(
        self, conversation_id: str, user_email: str
    ) -> list[Message]:
        conv = self._get_owned(conversation_id, user_email)
        return list(conv.messages.order_by("created_at"))

    def delete_conversation(self, conversation_id: str, user_email: str) -> None:
        conv = self._get_owned(conversation_id, user_email)
        conv.deleted_at = _now()
        conv.save(update_fields=["deleted_at", "updated_at"])

    def create_messages(
        self, conversation: Conversation, question: str
    ) -> Message:
        """Crea el user msg + assistant placeholder, devuelve el assistant.

        En MVP NO dispara el procesamiento async: lo hace el caller
        (view) llamando a `process_message` despues. Asi el front
        recibe 202 + message_id del assistant en pending, y empieza
        a pollearlo. Cuando se sume queue, esto se vuelve fire-and-
        forget.
        """
        now = _now()
        Message.objects.create(
            conversation=conversation,
            role=Message.ROLE_USER,
            content=question,
            status=Message.STATUS_COMPLETED,
            created_at=now,
        )
        assistant = Message.objects.create(
            conversation=conversation,
            role=Message.ROLE_ASSISTANT,
            status=Message.STATUS_PENDING,
            created_at=now,
        )
        if not conversation.title:
            conversation.title = _title_from_question(question)
        conversation.updated_at = now
        conversation.save(update_fields=["title", "updated_at"])
        return assistant

    def process_message(self, assistant: Message, question: str) -> None:
        """RAG + LLM. Persiste el resultado en `assistant`.

        Raises:
            LlmProviderError: si falla la llamada al provider.
        """
        start = time.monotonic()
        assistant.status = Message.STATUS_PROCESSING
        assistant.content_partial = "Buscando informacion relevante..."
        assistant.save(update_fields=["status", "content_partial"])

        try:
            context = self._retrieval.retrieve(question)
        except Exception as e:
            log.exception("chat: retrieval fallo")
            self._mark_error(assistant, f"Error en retrieval: {e}")
            return

        if context.is_empty:
            assistant.content = (
                "No encontre informacion relevante en el panel para "
                "responder tu pregunta. Proba reformularla o consulta "
                "el mantenedor correspondiente."
            )
            assistant.status = Message.STATUS_COMPLETED
            assistant.sources = []
            assistant.duration_ms = int((time.monotonic() - start) * 1000)
            assistant.save()
            return

        assistant.content_partial = "Generando respuesta..."
        assistant.save(update_fields=["content_partial"])

        history = self._build_history(assistant.conversation_id)
        system_prompt = build_system_prompt(context)
        user_message = build_user_message(question)
        messages = history + [LlmChatMessage(role="user", content=user_message)]

        try:
            response = self._llm.chat_complete(ChatRequest(
                messages=messages,
                system=system_prompt,
                max_tokens=2048,
                temperature=0.3,
            ))
        except LlmProviderError as e:
            log.exception("chat: llm fallo")
            self._mark_error(assistant, f"Error del LLM: {e}")
            return

        duration_ms = int((time.monotonic() - start) * 1000)
        assistant.content = response.content
        assistant.status = Message.STATUS_COMPLETED
        assistant.sources = [s.to_dict() for s in context.sources]
        assistant.tokens_in = response.usage.input_tokens
        assistant.tokens_out = response.usage.output_tokens
        assistant.model = response.model
        assistant.duration_ms = duration_ms
        assistant.content_partial = None
        assistant.save()

    def _mark_error(self, assistant: Message, error_message: str) -> None:
        assistant.status = Message.STATUS_ERROR
        assistant.error_message = error_message
        assistant.content = error_message
        assistant.content_partial = None
        assistant.save()

    def _get_owned(self, conversation_id: str, user_email: str) -> Conversation:
        try:
            return Conversation.objects.get(
                id=conversation_id,
                user_email=user_email,
                deleted_at__isnull=True,
            )
        except Conversation.DoesNotExist:
            raise PermissionError("Conversacion no encontrada") from None

    def _build_history(self, conversation_id) -> list[LlmChatMessage]:
        msgs = Message.objects.filter(
            conversation_id=conversation_id,
            status=Message.STATUS_COMPLETED,
            role__in=[Message.ROLE_USER, Message.ROLE_ASSISTANT],
        ).order_by("-created_at")[: CONTEXT_WINDOW + 1]
        out: list[LlmChatMessage] = []
        for m in reversed(list(msgs)):
            if m.content:
                out.append(LlmChatMessage(role=m.role, content=m.content))
        return out
