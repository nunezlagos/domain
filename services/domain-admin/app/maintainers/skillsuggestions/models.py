"""HU-52.3: modelo ORM managed=False contra la tabla `skill_suggestions`.

La tabla real la crea/migra domain-mcp via golang-migrate (mig 000182).
Django NO la crea ni la migra (`managed = False`): solo lee. Las mutaciones
de estado (approve/reject/apply) NO se hacen por ORM aca: se delegan al
endpoint REST del domain-mcp (ver services.py), que es la unica fuente de
verdad de la transicion + el audit_log (payload_hash SHA-256). Esto evita
duplicar la logica de negocio y el audit en dos lugares.

Esquema de skill_suggestions (mig 000182):
  id              UUID PK DEFAULT gen_random_uuid()
  skill_slug      TEXT NOT NULL
  kind            VARCHAR(10) CHECK (split|merge|refine|archive)
  payload         JSONB NOT NULL          (forma segun kind)
  rationale       TEXT (nullable)
  llm_model       TEXT (nullable)
  llm_confidence  DECIMAL(4,2) (nullable; 0..1)
  status          VARCHAR(20) DEFAULT 'pending'
                  CHECK (pending|approved|rejected|applied)
  reviewed_by     UUID (nullable)
  reviewed_at     TIMESTAMPTZ (nullable)
  applied_at      TIMESTAMPTZ (nullable)
  applied_changes JSONB (nullable)
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()

Single-tenant (regla dura 1): la tabla NO tiene organization_id.
"""
from __future__ import annotations

import uuid

from django.db import models
from django.utils import timezone


class SkillSuggestion(models.Model):
    """Una sugerencia del LLM-as-judge sobre un skill (human-in-the-loop)."""

    KIND_SPLIT = "split"
    KIND_MERGE = "merge"
    KIND_REFINE = "refine"
    KIND_ARCHIVE = "archive"
    KIND_CHOICES = [
        (KIND_SPLIT, "Split"),
        (KIND_MERGE, "Merge"),
        (KIND_REFINE, "Refine"),
        (KIND_ARCHIVE, "Archive"),
    ]

    STATUS_PENDING = "pending"
    STATUS_APPROVED = "approved"
    STATUS_REJECTED = "rejected"
    STATUS_APPLIED = "applied"
    STATUS_CHOICES = [
        (STATUS_PENDING, "Pendiente"),
        (STATUS_APPROVED, "Aprobada"),
        (STATUS_REJECTED, "Rechazada"),
        (STATUS_APPLIED, "Aplicada"),
    ]

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    skill_slug = models.TextField()
    kind = models.CharField(max_length=10, choices=KIND_CHOICES)
    payload = models.JSONField()
    rationale = models.TextField(null=True, blank=True)
    llm_model = models.TextField(null=True, blank=True)
    llm_confidence = models.DecimalField(
        max_digits=4, decimal_places=2, null=True, blank=True
    )
    status = models.CharField(
        max_length=20, choices=STATUS_CHOICES, default=STATUS_PENDING
    )
    reviewed_by = models.UUIDField(null=True, blank=True)
    reviewed_at = models.DateTimeField(null=True, blank=True)
    applied_at = models.DateTimeField(null=True, blank=True)
    applied_changes = models.JSONField(null=True, blank=True)
    created_at = models.DateTimeField(default=timezone.now)

    class Meta:
        db_table = "skill_suggestions"
        managed = False
        ordering = ["-created_at"]

    def __str__(self) -> str:
        return f"{self.kind} {self.skill_slug} [{self.status}]"

    # ── Predicados de estado (para los templates) ──────────────────────────
    @property
    def is_pending(self) -> bool:
        return self.status == self.STATUS_PENDING

    @property
    def is_approved(self) -> bool:
        return self.status == self.STATUS_APPROVED

    @property
    def is_applied(self) -> bool:
        return self.status == self.STATUS_APPLIED

    @property
    def is_rejected(self) -> bool:
        return self.status == self.STATUS_REJECTED

    @property
    def can_apply(self) -> bool:
        """approved y aun no aplicada => se puede aplicar (paso humano 2)."""
        return self.status == self.STATUS_APPROVED and self.applied_at is None

    @property
    def confidence_pct(self) -> int | None:
        if self.llm_confidence is None:
            return None
        return int(round(float(self.llm_confidence) * 100))
