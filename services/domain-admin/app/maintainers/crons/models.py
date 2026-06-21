"""Modelos del mantenedor de Crons (schedules user-defined), migrado a core.

Tabla existente en domain-mcp (managed=False, Django solo lee/escribe):
- crons: schedules definidos por el usuario que disparan un target
  (flow/agent/skill) según una expresión cron.

Cron hereda de core.models.SoftDeleteModel y reusa los campos comunes
(id / created_at / updated_at / deleted_at / status); declara SOLO sus
columnas propias. Las columnas declaradas deben matchear EXACTO la tabla real
`crons` (guard: core/tests/test_schema_drift.py + real_schema.json).

NO confundir con system_crons (crons internos del sistema).
"""
from __future__ import annotations

from django.db import models

from core.models import SoftDeleteModel


class Cron(SoftDeleteModel):
    """Cron schedule de la plataforma.

    id / created_at / updated_at / deleted_at / status vienen de SoftDeleteModel.
    `status` se redeclara solo para matchear el tipo real de la columna (en la
    BD es `text`, no varchar(20) como el default del abstracto).

    Schema real (crons), columna por columna (information_schema):
        id              uuid PK default gen_random_uuid()      (SoftDeleteModel)
        created_by      uuid NULL FK users(id) ON DELETE SET NULL
        slug            varchar(100) NOT NULL
        name            varchar(255) NOT NULL
        description     text NULL
        cron_expression varchar(100) NOT NULL
        timezone        varchar(50) NOT NULL default 'UTC'
        target_type     varchar(20) NOT NULL CHECK IN ('flow','agent','skill')
        target_id       uuid NOT NULL
        inputs          jsonb NOT NULL default '{}'
        enabled         boolean NOT NULL default true
        last_run_at     timestamptz NULL
        next_run_at     timestamptz NULL
        created_at      timestamptz NOT NULL default now()      (SoftDeleteModel)
        updated_at      timestamptz NOT NULL default now()       (SoftDeleteModel)
        deleted_at      timestamptz NULL                         (SoftDeleteModel)
        status          text NOT NULL default 'active'           (SoftDeleteModel)

    `organization_id` FUE DROPEADA (fase C, migración 000142); NO existe más.

    `enabled` es la dimensión alternable (toggle on/off); el display de
    estado se deriva del bool (`is_active`).
    """

    # target_type es un CHECK, no un status; lo exponemos como choices del
    # campo target_type para validar el form y para get_target_type_display.
    TARGET_TYPE_CHOICES = [
        ("flow", "Flow"),
        ("agent", "Agent"),
        ("skill", "Skill"),
    ]

    created_by = models.UUIDField(null=True, blank=True)
    slug = models.CharField(max_length=100)
    name = models.CharField(max_length=255)
    description = models.TextField(blank=True, default="")
    cron_expression = models.CharField(max_length=100)
    timezone = models.CharField(max_length=50, default="UTC")
    target_type = models.CharField(
        max_length=20, default="flow", choices=TARGET_TYPE_CHOICES
    )
    target_id = models.UUIDField()
    inputs = models.JSONField(default=dict, blank=True)
    enabled = models.BooleanField(default=True)
    last_run_at = models.DateTimeField(null=True, blank=True)
    next_run_at = models.DateTimeField(null=True, blank=True)
    # Redeclara status (heredado de SoftDeleteModel) solo para matchear el tipo
    # real de la columna: en `crons` es `text`, no varchar(20).
    status = models.TextField(default="active")

    class Meta:
        db_table = "crons"
        managed = False
        ordering = ["-created_at"]

    def __str__(self) -> str:
        return f"{self.name} ({self.slug})"

    @property
    def display_name(self) -> str:
        return self.name or self.slug

    @property
    def is_active(self) -> bool:
        return self.enabled and self.deleted_at is None
