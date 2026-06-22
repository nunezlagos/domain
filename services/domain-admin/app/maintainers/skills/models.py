"""Modelos del mantenedor de Skills (migrado a core).

2 tablas existentes en domain-mcp (managed=False, Django solo lee/escribe):
- skills:         definicion de una skill   -> hereda de core.SoftDeleteModel
- skill_versions: snapshots inmutables       -> hereda de core.BaseModel (read-only)

Skill reusa los campos comunes (id/created_at/updated_at/deleted_at/status) de
core.models.SoftDeleteModel; SkillVersion reusa (id/created_at/updated_at) de
core.models.BaseModel. Cada uno declara SOLO sus columnas propias. Las columnas
declaradas deben matchear EXACTO la tabla real (guard:
core/tests/test_schema_drift.py + real_schema.json).

Evolucion del schema relevante (verificada en migrations):
- 000056: skills.pinned_version INT nullable.
- 000107: skills.project_id UUID nullable (NULL = skill global de la org;
  not-NULL = skill de un proyecto). El UNIQUE paso a ser por (project_id, slug)
  via 2 indices parciales (global vs por proyecto).
- 000110: skills.proposed BOOL default false.
- 000142: DROP COLUMN organization_id en TODAS las tablas (incluida skills).
- 000144: skill_type deprecado a 'prompt' para api/code/mcp_tool, pero el CHECK
  sigue aceptando los 4 valores.

skills SI tiene `status` y `deleted_at` (soft-delete). La baja es soft-delete
(deleted_at); el toggle de estado NO se expone en el mantenedor (la baja real es
deleted_at, no un status alternable).

Columnas DB-managed omitidas del modelo (no se mapean, igual que antes):
- description_tsv: TSVECTOR GENERATED ALWAYS (la calcula la BD).
- embedding: vector(1536) (pgvector, sin field nativo de Django).
"""
from __future__ import annotations

from django.contrib.postgres.fields import ArrayField
from django.db import models

from core.models import BaseModel, SoftDeleteModel


class Skill(SoftDeleteModel):
    """Skill de la plataforma.

    id / created_at / updated_at / deleted_at / status vienen de SoftDeleteModel.
    Las columnas declaradas aqui matchean EXACTO la tabla real `skills`.
    """

    # skill_type no es un "status" alternable; lo usamos como choices para
    # habilitar {{ skill.get_skill_type_display }} en templates. Los valores
    # coinciden con el CHECK real de la tabla.
    SKILL_TYPE_CHOICES = [
        ("prompt", "Prompt"),
        ("code", "Codigo"),
        ("api", "API"),
        ("mcp_tool", "Herramienta MCP"),
    ]

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
        """Una skill esta vigente si no esta soft-deleted ni propuesta."""
        return self.deleted_at is None and not self.proposed


class SkillVersion(BaseModel):
    """Snapshot inmutable de una skill por version. Read-only desde el admin.

    id / created_at / updated_at vienen de BaseModel. `status` existe en la
    tabla real (skill_versions) pero NO `deleted_at`, por eso hereda de
    BaseModel (no SoftDeleteModel) y declara `status` como columna propia.
    """

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
    status = models.CharField(max_length=20, default="active")

    class Meta:
        db_table = "skill_versions"
        managed = False
        ordering = ["-version"]
        unique_together = [("skill", "version")]

    def __str__(self) -> str:
        return f"{self.skill_id} v{self.version}"
