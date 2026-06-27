"""HU-52.1: modelo ORM managed=False contra la tabla `skill_feedback`.

La tabla real la crea/migra domain-mcp via golang-migrate (mig 000180).
Django NO la crea ni la migra (`managed = False`): solo lee/escribe via ORM.
Las columnas declaradas aca deben matchear EXACTO la tabla real; cualquier
drift rompe en runtime con `django.db.utils.ProgrammingError`.

Esquema de skill_feedback (mig 000180):
  id          UUID PK DEFAULT gen_random_uuid()
  message_id  BIGINT NOT NULL REFERENCES chat_messages(id)  (sin cascade)
  skill_slug  TEXT (nullable; extraido del source del mensaje, NO FK)
  rating      SMALLINT NOT NULL CHECK (rating IN (1, -1))
  comment     TEXT (nullable; puede tener PII -> NUNCA loguear)
  user_email  TEXT (nullable)
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()  (trigger set_updated_at)
  UNIQUE(message_id)  -> 1 feedback por mensaje (idempotencia/upsert)

Single-tenant: la tabla NO tiene organization_id (regla dura 1).
"""
from __future__ import annotations

import uuid

from django.db import models
from django.utils import timezone


class SkillFeedback(models.Model):
    """Un voto 👍/👎 del usuario sobre una respuesta del assistant."""

    RATING_UP = 1
    RATING_DOWN = -1
    RATING_CHOICES = [
        (RATING_UP, "Positivo"),
        (RATING_DOWN, "Negativo"),
    ]

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    # message_id matchea chat_messages.id (BigAutoField/BIGINT). No declaramos
    # una FK ORM: chat_messages vive en otra app (chat) y solo necesitamos el
    # entero para el upsert; ademas la FK en DB no tiene cascade a proposito.
    message_id = models.BigIntegerField()
    skill_slug = models.TextField(null=True, blank=True)
    rating = models.SmallIntegerField(choices=RATING_CHOICES)
    comment = models.TextField(null=True, blank=True)
    user_email = models.TextField(null=True, blank=True)
    created_at = models.DateTimeField(default=timezone.now)
    updated_at = models.DateTimeField(default=timezone.now)

    class Meta:
        db_table = "skill_feedback"
        managed = False
        ordering = ["-created_at"]

    def __str__(self) -> str:
        sign = "+1" if self.rating == self.RATING_UP else "-1"
        return f"feedback {sign} msg#{self.message_id}"

    @property
    def is_positive(self) -> bool:
        return self.rating == self.RATING_UP


# NOTA: la tabla `skill_feedback_daily` (mig 000180) la ESCRIBE el aggregator
# del domain-mcp (cron cada 6h) y tiene PK compuesta (skill_slug, day), que el
# ORM de Django no modela limpio. Como el Django solo necesita LEER agregados,
# lo hace via SQL crudo en `services.aggregate_by_skill()` (self-contained,
# computa los conteos directo desde skill_feedback). No declaramos un modelo
# managed=False con PK falsa para evitar drift de schema bajo el test-runner.
# En HU-52.2 (skill_metrics) se decidira si se expone como modelo de lectura.
