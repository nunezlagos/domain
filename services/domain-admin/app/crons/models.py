"""Modelos del mantenedor de Crons (schedules user-defined).

Tabla existente en domain-mcp (migración 000016_create_crons):
- crons: schedules definidos por el usuario que disparan un target
  (flow/agent/skill) según una expresión cron. Aislada por
  organization_id (multi-tenant). Soft-delete vía deleted_at + flag
  booleano `enabled` para habilitar/deshabilitar (toggle).

NO confundir con system_crons (crons internos del sistema).

Django NO migra esta tabla (managed=False). Solo lee/escribe vía ORM;
las filas (incluido el PK uuid) las genera domain-mcp en producción.
"""
import uuid

from django.db import models


class Cron(models.Model):
    """Cron schedule de la plataforma. PK uuid.

    Schema real (crons), columna por columna:
        id              uuid PK default gen_random_uuid()
        organization_id uuid NOT NULL FK organizations(id)
        created_by      uuid NULL FK users(id) ON DELETE SET NULL
        slug            varchar(100) NOT NULL  (unique per organization_id)
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
        created_at      timestamptz NOT NULL default now()
        updated_at      timestamptz NOT NULL default now()  (trigger set_updated_at)
        deleted_at      timestamptz NULL
        UNIQUE (organization_id, slug)

    `enabled` es la dimensión alternable (toggle on/off); NO hay columna
    `status` con choices, así que el display de estado se deriva del bool.
    """

    # target_type es un CHECK, no un status; lo exponemos como choices del
    # campo target_type para validar el form y para get_target_type_display.
    TARGET_TYPE_CHOICES = [
        ("flow", "Flow"),
        ("agent", "Agent"),
        ("skill", "Skill"),
    ]

    id = models.UUIDField(primary_key=True, default=uuid.uuid4)
    organization_id = models.UUIDField()
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
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
    deleted_at = models.DateTimeField(null=True, blank=True)

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
