"""Modelos del mantenedor de Agentes (migrado a core).

Tablas existentes en domain-mcp (managed=False; Django NO las migra, solo
lee/escribe via ORM). Las filas (incluido el PK uuid) las genera domain-mcp.

- agents:         entidad principal. Tiene deleted_at + status -> hereda de
                  core.SoftDeleteModel (reusa id/created_at/updated_at/
                  deleted_at/status; redeclara status solo para choices).
- agent_versions: historial (snapshot JSONB). PK BIGSERIAL (no uuid), por eso
                  NO hereda de core.BaseModel (que impone PK uuid). READ-ONLY.
                  Excluida del guard via _SKIP_TABLES no aplica: la tabla SI
                  esta en real_schema; declaramos solo columnas existentes.
- agent_templates: catalogo reutilizable. Tiene id uuid + created_at +
                  updated_at pero NO deleted_at -> hereda de core.BaseModel
                  (reusa id/created_at/updated_at; agrega status propio).
                  READ-ONLY.

Cada columna declarada debe existir EXACTO en la tabla real (guard:
core/tests/test_schema_drift.py + real_schema.json). organization_id fue
DROPEADA por la migracion 000142; el slug es unico globalmente.
"""
from __future__ import annotations

import uuid

from django.contrib.postgres.fields import ArrayField
from django.db import models

from core.models import BaseModel, SoftDeleteModel


class Agent(SoftDeleteModel):
    """Agente LLM. Entidad principal del mantenedor.

    id / created_at / updated_at / deleted_at / status vienen de
    SoftDeleteModel. `status` se redeclara solo para sumarle choices (misma
    columna). El resto son columnas propias de la tabla `agents`.
    """

    STATUS_CHOICES = [
        ("active", "Activo"),
        ("inactive", "Inactivo"),
    ]

    slug = models.CharField(max_length=100)
    name = models.CharField(max_length=255)
    description = models.TextField(blank=True, default="")
    provider = models.CharField(max_length=50)
    model = models.CharField(max_length=100)
    system_prompt = models.TextField(blank=True, default="")
    skills_slugs = ArrayField(
        models.CharField(max_length=255), default=list, blank=True
    )
    max_iterations = models.IntegerField(default=20)
    token_budget = models.BigIntegerField(null=True, blank=True)
    temperature = models.DecimalField(
        max_digits=3, decimal_places=2, null=True, blank=True
    )
    seed_managed = models.BooleanField(default=False)
    seed_version = models.IntegerField(null=True, blank=True)
    is_user_modified = models.BooleanField(default=False)
    # Redeclara status (heredado de SoftDeleteModel) solo para agregar choices.
    status = models.CharField(max_length=20, default="active", choices=STATUS_CHOICES)

    class Meta:
        db_table = "agents"
        managed = False
        ordering = ["-created_at"]

    def __str__(self) -> str:
        return f"{self.name} ({self.slug})"

    @property
    def display_name(self) -> str:
        return self.name or self.slug

    @property
    def is_active(self) -> bool:
        # Activo = status 'active' y sin soft-delete.
        return self.status == "active" and self.deleted_at is None


class AgentVersion(models.Model):
    """Version historica de un Agent (snapshot JSONB). READ-ONLY en el admin.

    PK BIGSERIAL (no uuid) -> no puede heredar de core.BaseModel (PK uuid). Se
    declaran solo las columnas que el admin consume; status/updated_at existen
    en la tabla pero no se declaran (READ-ONLY, managed=False, no rompe nada).
    """

    id = models.BigAutoField(primary_key=True)
    agent = models.ForeignKey(
        Agent,
        on_delete=models.CASCADE,
        db_column="agent_id",
        related_name="versions",
    )
    version = models.IntegerField()
    snapshot = models.JSONField(default=dict, blank=True)
    changed_by = models.UUIDField(null=True, blank=True)
    created_at = models.DateTimeField(auto_now_add=True)

    class Meta:
        db_table = "agent_versions"
        managed = False
        ordering = ["-version"]

    def __str__(self) -> str:
        return f"{self.agent_id} v{self.version}"


class AgentTemplate(BaseModel):
    """Template reutilizable de agente (personality + handoff). READ-ONLY.

    id / created_at / updated_at vienen de core.BaseModel. NO tiene deleted_at
    (por eso BaseModel y no SoftDeleteModel); `status` SI existe en la tabla y
    se declara aqui (BaseModel no lo aporta).
    """

    HANDOFF_POLICY_CHOICES = [
        ("allow", "Permitir"),
        ("forbid", "Prohibir"),
        ("require_supervisor_approval", "Requiere aprobacion de supervisor"),
    ]
    ROLE_CHOICES = [
        ("orchestrator", "Orquestador"),
        ("phase-worker", "Phase worker"),
    ]
    STATUS_CHOICES = [
        ("active", "Activo"),
        ("inactive", "Inactivo"),
    ]

    slug = models.CharField(max_length=80)
    name = models.CharField(max_length=120)
    system_prompt = models.TextField()
    personality = models.TextField(blank=True, default="")
    capabilities = ArrayField(
        models.CharField(max_length=255), default=list, blank=True
    )
    model = models.CharField(max_length=80, default="claude-haiku-4-5")
    temperature = models.DecimalField(max_digits=3, decimal_places=2, default=0.7)
    max_tokens = models.IntegerField(default=4096)
    handoff_policy = models.CharField(
        max_length=40, default="allow", choices=HANDOFF_POLICY_CHOICES
    )
    metadata = models.JSONField(default=dict, blank=True)
    created_by = models.UUIDField(null=True, blank=True)
    role = models.CharField(max_length=20, default="phase-worker", choices=ROLE_CHOICES)
    seed_managed = models.BooleanField(default=False)
    is_user_modified = models.BooleanField(default=False)
    seed_version = models.IntegerField(null=True, blank=True)
    status = models.CharField(max_length=20, default="active", choices=STATUS_CHOICES)

    class Meta:
        db_table = "agent_templates"
        managed = False
        ordering = ["name"]

    def __str__(self) -> str:
        return f"{self.name} ({self.slug})"

    @property
    def display_name(self) -> str:
        return self.name or self.slug
