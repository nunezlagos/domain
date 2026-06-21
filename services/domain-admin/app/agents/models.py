"""Modelos del mantenedor de Agentes.

Tablas existentes en domain-mcp (managed=False; Django NO las migra, solo
lee/escribe vía ORM). Las filas (incluido el PK uuid) las genera domain-mcp
en producción.

- agents (migración 000012 + 000120 + 000142): definición de un agente LLM
  (provider/model/system_prompt/skills/límites). 000142 DROPEÓ
  organization_id (la tabla organizations ya no existe). 000120 agregó
  `status` (TEXT NOT NULL DEFAULT 'active'). Soft-delete vía deleted_at
  (la columna sigue existiendo).
- agent_versions (migración 000083_create_agent_versions): historial de
  versiones de un agent con snapshot JSONB. PK BIGSERIAL, FK agent_id.
  Nunca tuvo organization_id. READ-ONLY desde el admin (se listan en el
  detalle del agent).
- agent_templates (migración 000068 + 000075 + 000120 + 000142):
  definitions reutilizables de agents (personality + handoff policy + role).
  000142 DROPEÓ organization_id. 000120 agregó `status`. NO tiene
  deleted_at. READ-ONLY desde el admin (catálogo en el detalle del agent).
"""
import uuid

from django.contrib.postgres.fields import ArrayField
from django.db import models


class Agent(models.Model):
    """Agente LLM. PK uuid. Entidad principal del mantenedor.

    Schema real (agents, tras 000120 + 000142):
        id              uuid PK default gen_random_uuid()
        slug            varchar(100) NOT NULL
        name            varchar(255) NOT NULL
        description     text NULL
        provider        varchar(50) NOT NULL
        model           varchar(100) NOT NULL
        system_prompt   text NULL
        skills_slugs    text[] NOT NULL default '{}'
        max_iterations  int NOT NULL default 20
        token_budget    bigint NULL
        temperature     numeric(3,2) NULL
        seed_managed    boolean NOT NULL default false
        seed_version    int NULL
        is_user_modified boolean NOT NULL default false
        created_at      timestamptz NOT NULL default now()
        updated_at      timestamptz NOT NULL default now()  (trigger set_updated_at)
        deleted_at      timestamptz NULL
        status          text NOT NULL default 'active'

    organization_id fue DROPEADA por la migración 000142 (la tabla
    organizations ya no existe); el slug ya NO es único per-org. Soft-delete
    SÍ aplica (existe deleted_at). status existe (000120, free-text default
    'active').
    """

    STATUS_CHOICES = [
        ("active", "Activo"),
        ("inactive", "Inactivo"),
    ]

    id = models.UUIDField(primary_key=True, default=uuid.uuid4)
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
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
    deleted_at = models.DateTimeField(null=True, blank=True)
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
    """Versión histórica de un Agent (snapshot JSONB). READ-ONLY en el admin.

    Schema real (agent_versions):
        id          bigserial PK
        agent_id    uuid NOT NULL FK agents(id) ON DELETE CASCADE
        version     int NOT NULL
        snapshot    jsonb NOT NULL default '{}'
        changed_by  uuid NULL
        created_at  timestamptz NOT NULL default now()
        UNIQUE (agent_id, version)

    Nunca tuvo organization_id. 000120 le agregó status/updated_at, pero el
    admin no los usa (READ-ONLY): no se declaran fields para columnas no
    consumidas, y al ser managed=False no rompe nada.
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


class AgentTemplate(models.Model):
    """Template reutilizable de agente (personality + handoff). READ-ONLY.

    Schema real (agent_templates, tras 000075 + 000120 + 000142):
        id               uuid PK default gen_random_uuid()
        slug             varchar(80) NOT NULL
        name             varchar(120) NOT NULL
        system_prompt    text NOT NULL
        personality      text NULL
        capabilities     text[] NOT NULL default '{}'
        model            varchar(80) NOT NULL default 'claude-haiku-4-5'
        temperature      numeric(3,2) NOT NULL default 0.7
        max_tokens       int NOT NULL default 4096
        handoff_policy   varchar(40) NOT NULL default 'allow'
                         CHECK IN ('allow','forbid','require_supervisor_approval')
        metadata         jsonb NOT NULL default '{}'
        created_by       uuid NULL FK users(id) ON DELETE SET NULL
        created_at       timestamptz NOT NULL default now()
        updated_at       timestamptz NOT NULL default now()  (trigger)
        role             varchar(20) NOT NULL default 'phase-worker'
                         CHECK IN ('orchestrator','phase-worker')
        seed_managed     boolean NOT NULL default false
        is_user_modified boolean NOT NULL default false
        seed_version     int NULL
        status           text NOT NULL default 'active'

    organization_id fue DROPEADA por 000142. status existe (000120). NO tiene
    deleted_at.
    """

    HANDOFF_POLICY_CHOICES = [
        ("allow", "Permitir"),
        ("forbid", "Prohibir"),
        ("require_supervisor_approval", "Requiere aprobación de supervisor"),
    ]
    ROLE_CHOICES = [
        ("orchestrator", "Orquestador"),
        ("phase-worker", "Phase worker"),
    ]

    STATUS_CHOICES = [
        ("active", "Activo"),
        ("inactive", "Inactivo"),
    ]

    id = models.UUIDField(primary_key=True, default=uuid.uuid4)
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
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
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
