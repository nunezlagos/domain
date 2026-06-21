"""Modelos del mantenedor de Skills.

Tablas existentes en domain-mcp:
- skills        (000010_create_skills + alters): definición de una skill.
- skill_versions (000011_create_skill_versions): snapshots inmutables por
  versión de una skill. Read-only desde el admin (no CRUD).

Django NO migra estas tablas (managed=False). Solo lee/escribe vía ORM; las
filas (incluido el PK uuid) las genera domain-mcp en producción.

Evolución del schema relevante (verificada en migrations):
- 000056: skills.pinned_version INT nullable.
- 000107: skills.project_id UUID nullable (NULL = skill global de la org;
  not-NULL = skill de un proyecto). El UNIQUE pasó a ser por (project_id, slug)
  vía 2 índices parciales (global vs por proyecto).
- 000110: skills.proposed BOOL default false.
- 000142: DROP COLUMN organization_id en TODAS las tablas (incluida skills).
  Por eso skills NO tiene organization_id en el modelo.
- 000144: skill_type deprecado a 'prompt' para api/code/mcp_tool, pero el
  CHECK sigue aceptando los 4 valores (no se dropeó la columna ni el CHECK).

skills NO tiene columna `status` → NO hay toggle de estado. SÍ tiene
`deleted_at` → soft-delete.

Columnas DB-managed omitidas del modelo (no se mapean):
- description_tsv: TSVECTOR GENERATED ALWAYS (la calcula la BD).
- embedding: vector(1536) (pgvector, sin field nativo de Django).
"""
import uuid

from django.contrib.postgres.fields import ArrayField
from django.db import models


class Skill(models.Model):
    """Skill de la plataforma. PK uuid.

    Schema real (skills) — columnas mapeadas:
        id              uuid PK default gen_random_uuid()
        slug            varchar(100) NOT NULL  (único por project_id)
        name            varchar(255) NOT NULL
        description     text NULL
        skill_type      varchar(20) NOT NULL  CHECK in (prompt/code/api/mcp_tool)
        content         text NULL
        input_schema    jsonb NOT NULL default '{}'
        output_schema   jsonb NOT NULL default '{}'
        timeout_seconds int NOT NULL default 30  CHECK between 1 and 600
        idempotent      bool NOT NULL default false
        has_side_effects bool NOT NULL default false
        depends_on      text[] NOT NULL default '{}'
        tags            text[] NOT NULL default '{}'
        seed_managed    bool NOT NULL default false
        seed_version    int NULL
        is_user_modified bool NOT NULL default false
        pinned_version  int NULL                 (000056)
        project_id      uuid NULL FK projects(id) (000107)
        proposed        bool NOT NULL default false (000110)
        created_at      timestamptz NOT NULL default now()
        updated_at      timestamptz NOT NULL default now()  (trigger set_updated_at)
        deleted_at      timestamptz NULL
    """

    # skill_type no es un "status" alternable; lo usamos como choices para
    # habilitar {{ skill.get_skill_type_display }} en templates. Los valores
    # coinciden con el CHECK real de la tabla.
    SKILL_TYPE_CHOICES = [
        ("prompt", "Prompt"),
        ("code", "Código"),
        ("api", "API"),
        ("mcp_tool", "Herramienta MCP"),
    ]

    id = models.UUIDField(primary_key=True, default=uuid.uuid4)
    slug = models.CharField(max_length=100)
    name = models.CharField(max_length=255)
    description = models.TextField(blank=True, default="")
    skill_type = models.CharField(
        max_length=20, default="prompt", choices=SKILL_TYPE_CHOICES
    )
    content = models.TextField(blank=True, default="")
    input_schema = models.JSONField(default=dict, blank=True)
    output_schema = models.JSONField(default=dict, blank=True)
    timeout_seconds = models.IntegerField(default=30)
    idempotent = models.BooleanField(default=False)
    has_side_effects = models.BooleanField(default=False)
    # Columnas Postgres text[] → ArrayField (NO JSONField).
    depends_on = ArrayField(models.CharField(max_length=255), default=list, blank=True)
    tags = ArrayField(models.CharField(max_length=100), default=list, blank=True)
    seed_managed = models.BooleanField(default=False)
    seed_version = models.IntegerField(null=True, blank=True)
    is_user_modified = models.BooleanField(default=False)
    pinned_version = models.IntegerField(null=True, blank=True)
    project_id = models.UUIDField(null=True, blank=True)
    proposed = models.BooleanField(default=False)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
    deleted_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "skills"
        managed = False
        ordering = ["-created_at"]

    def __str__(self) -> str:
        return f"{self.name} ({self.slug})"

    @property
    def display_name(self) -> str:
        return self.name or self.slug

    @property
    def is_active(self) -> bool:
        """Una skill está vigente si no está soft-deleted ni propuesta."""
        return self.deleted_at is None and not self.proposed


class SkillVersion(models.Model):
    """Snapshot inmutable de una skill por versión. Read-only desde el admin.

    Schema real (skill_versions):
        id          uuid PK default gen_random_uuid()
        skill_id    uuid NOT NULL FK skills(id) ON DELETE CASCADE
        version     int NOT NULL
        content     text NULL
        input_schema  jsonb NULL
        output_schema jsonb NULL
        changelog   text NULL
        created_by  uuid NULL FK users(id) ON DELETE SET NULL
        created_at  timestamptz NOT NULL default now()
        UNIQUE (skill_id, version)
    """

    id = models.UUIDField(primary_key=True, default=uuid.uuid4)
    skill = models.ForeignKey(
        Skill,
        on_delete=models.CASCADE,
        db_column="skill_id",
        related_name="versions",
    )
    version = models.IntegerField()
    content = models.TextField(null=True, blank=True)
    input_schema = models.JSONField(null=True, blank=True)
    output_schema = models.JSONField(null=True, blank=True)
    changelog = models.TextField(null=True, blank=True)
    created_by = models.UUIDField(null=True, blank=True)
    created_at = models.DateTimeField(auto_now_add=True)

    class Meta:
        db_table = "skill_versions"
        managed = False
        ordering = ["-version"]
        unique_together = [("skill", "version")]

    def __str__(self) -> str:
        return f"{self.skill_id} v{self.version}"
