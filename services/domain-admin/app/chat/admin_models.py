"""HU-49.2: ORM models managed=False contra las tablas de domain-mcp.

Las tablas reales (chat_conversations, chat_messages, chat_document_embeddings)
viven en domain-mcp y son migradas por golang-migrate (mig 000171). Django
NO las crea ni las migra (`managed = False`); solo lee/escribe via ORM.

Las columnas declaradas aca deben matchear EXACTO la tabla real. Cualquier
drift rompe en runtime con `django.db.utils.ProgrammingError`. El guard
centralizado en `core/tests/test_schema_drift.py` valida el resto de las
tablas; las del chat se validan con un guard equivalente (TODO cuando
se sume al sistema de drift).
"""
from __future__ import annotations

import uuid

from django.db import models
from django.utils import timezone


class Conversation(models.Model):
    """Conversacion de chat del admin.

    PK uuid (la crea domain-mcp via gen_random_uuid). Soft delete via
    `deleted_at`. `title` lo setea el ChatService en el primer mensaje
    (truncado a 60 chars).
    """

    id = models.UUIDField(primary_key=True, default=uuid.uuid4)
    user_email = models.TextField()
    title = models.TextField(default="")
    created_at = models.DateTimeField(default=timezone.now)
    updated_at = models.DateTimeField(default=timezone.now)
    deleted_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "chat_conversations"
        managed = False
        ordering = ["-updated_at"]

    def __str__(self) -> str:
        return self.title or f"Conversation {self.id}"


class Message(models.Model):
    """Mensaje individual de una conversacion.

    `role` es 'user' (pregunta) o 'assistant' (respuesta del LLM).
    `status` controla el ciclo de vida de los mensajes assistant:
    pending (esperando RAG) -> processing (RAG en curso) -> completed/error.
    `sources` es JSONB con la lista de fuentes citadas en la respuesta.
    """

    STATUS_PENDING = "pending"
    STATUS_PROCESSING = "processing"
    STATUS_COMPLETED = "completed"
    STATUS_ERROR = "error"
    STATUS_CHOICES = [
        (STATUS_PENDING, "Pending"),
        (STATUS_PROCESSING, "Processing"),
        (STATUS_COMPLETED, "Completed"),
        (STATUS_ERROR, "Error"),
    ]
    ROLE_USER = "user"
    ROLE_ASSISTANT = "assistant"

    id = models.BigAutoField(primary_key=True)
    conversation = models.ForeignKey(
        Conversation,
        on_delete=models.CASCADE,
        db_column="conversation_id",
        related_name="messages",
    )
    role = models.TextField()
    content = models.TextField(null=True, blank=True)
    content_partial = models.TextField(null=True, blank=True)
    status = models.TextField(default=STATUS_PENDING)
    sources = models.JSONField(default=list, blank=True)
    tokens_in = models.IntegerField(default=0)
    tokens_out = models.IntegerField(default=0)
    model = models.TextField(default="")
    duration_ms = models.IntegerField(default=0)
    error_message = models.TextField(null=True, blank=True)
    created_at = models.DateTimeField(default=timezone.now)

    class Meta:
        db_table = "chat_messages"
        managed = False
        ordering = ["created_at"]

    def __str__(self) -> str:
        return f"{self.role}#{self.id}"


class DocumentEmbedding(models.Model):
    """Cache de embeddings para RAG.

    `source_table` + `source_id` + `chunk_index` forman la clave unica.
    `valid_until` permite invalidar cache por TTL (ej: proyectos que
    cambian seguido). Por ahora se cachea permanente (valid_until NULL).
    """

    id = models.BigAutoField(primary_key=True)
    source_table = models.TextField()
    source_id = models.UUIDField()
    source_url = models.TextField(default="")
    chunk_text = models.TextField()
    chunk_index = models.IntegerField(default=0)
    model = models.TextField(default="")
    created_at = models.DateTimeField(default=timezone.now)
    valid_until = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "chat_document_embeddings"
        managed = False
        unique_together = [("source_table", "source_id", "chunk_index")]

    def __str__(self) -> str:
        return f"{self.source_table}#{self.source_id}#{self.chunk_index}"