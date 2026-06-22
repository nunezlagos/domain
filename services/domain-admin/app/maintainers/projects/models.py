"""Modelos del mantenedor de Proyectos (migrado a core).

3 tablas existentes en domain-mcp (managed=False, Django solo lee/escribe vía
ORM; las filas y el PK uuid los genera domain-mcp en prod):

- projects:             proyecto. Soft-delete vía deleted_at + status
                        -> hereda de core.SoftDeleteModel.
- project_templates:    templates preconfigurados. SIN deleted_at (solo status),
                        catálogo seeded de solo-lectura -> hereda de core.BaseModel
                        y declara `status` propio.
- project_repositories: N remotos git por proyecto. Soft-delete vía deleted_at +
                        status + flag is_default -> hereda de core.SoftDeleteModel.

Cada model reusa los campos comunes (id/created_at/updated_at, y deleted_at/status
en SoftDeleteModel) de core.models y declara SOLO sus columnas propias. Las
columnas declaradas matchean EXACTO el schema real (guard:
core/tests/test_schema_drift.py + core/tests/real_schema.json).

NOTA: la tabla `organizations` fue eliminada y la columna `organization_id` fue
dropeada de estas tablas. Por eso NINGÚN modelo declara organization_id.
"""
from __future__ import annotations

from django.contrib.postgres.fields import ArrayField
from django.db import models

from core.models import BaseModel, SoftDeleteModel


class ProjectTemplate(BaseModel):
    """Template de proyecto. Catálogo seeded, solo-lectura desde el admin.

    id / created_at / updated_at vienen de BaseModel. NO tiene deleted_at, por
    eso NO hereda de SoftDeleteModel; declara `status` propio (la columna existe
    en la tabla pero no hay soft delete).
    """

    slug = models.CharField(max_length=100)
    name = models.CharField(max_length=255)
    description = models.TextField(blank=True, default="")
    is_default = models.BooleanField(default=False)
    is_public = models.BooleanField(default=False)
    settings = models.JSONField(default=dict, blank=True)
    default_skills = ArrayField(models.CharField(max_length=100), default=list, blank=True)
    default_agents = ArrayField(models.CharField(max_length=100), default=list, blank=True)
    default_flows = ArrayField(models.CharField(max_length=100), default=list, blank=True)
    seed_managed = models.BooleanField(default=False)
    seed_version = models.IntegerField(null=True, blank=True)
    is_user_modified = models.BooleanField(default=False)
    status = models.CharField(max_length=20, default="active")

    class Meta:
        db_table = "project_templates"
        managed = False
        ordering = ["slug"]

    def __str__(self) -> str:
        return f"{self.name} ({self.slug})"


class Project(SoftDeleteModel):
    """Proyecto. Soft-delete vía deleted_at + columna status.

    id / created_at / updated_at / deleted_at / status vienen de SoftDeleteModel.
    `status` se redeclara solo para sumarle choices (misma columna).
    """

    STATUS_ACTIVE = "active"
    STATUS_ARCHIVED = "archived"
    STATUS_CHOICES = [
        (STATUS_ACTIVE, "Activo"),
        (STATUS_ARCHIVED, "Archivado"),
    ]

    name = models.CharField(max_length=255)
    slug = models.CharField(max_length=100)
    description = models.TextField(blank=True, default="")
    repository_url = models.CharField(max_length=500, blank=True, default="")
    template_id = models.UUIDField(null=True, blank=True)
    settings = models.JSONField(default=dict, blank=True)
    current_branch = models.CharField(max_length=120, blank=True, default="")
    # En la DB es text[] (no jsonb), por eso ArrayField.
    rules = ArrayField(models.CharField(max_length=100), default=list, blank=True)
    client_id = models.UUIDField(null=True, blank=True)
    last_known_head = models.CharField(max_length=40, blank=True, default="")
    last_seen_at = models.DateTimeField(null=True, blank=True)
    last_seen_branch = models.CharField(max_length=120, blank=True, default="")
    last_seen_cwd = models.CharField(max_length=500, blank=True, default="")
    # Redeclara status (heredado de SoftDeleteModel) solo para agregar choices.
    status = models.CharField(
        max_length=20, default=STATUS_ACTIVE, choices=STATUS_CHOICES
    )

    class Meta:
        db_table = "projects"
        managed = False
        ordering = ["-created_at"]

    def __str__(self) -> str:
        return f"{self.name} ({self.slug})"

    @property
    def display_name(self) -> str:
        return self.name or self.slug

    @property
    def is_active(self) -> bool:
        return self.deleted_at is None


class ProjectRepository(SoftDeleteModel):
    """Remoto git de un proyecto. Soft-delete vía deleted_at + flag is_default.

    id / created_at / updated_at / deleted_at / status vienen de SoftDeleteModel.
    """

    KIND_CHOICES = [
        ("github", "GitHub"),
        ("gitlab", "GitLab"),
        ("bitbucket", "Bitbucket"),
        ("gitea", "Gitea"),
        ("other", "Otro"),
    ]
    WORKFLOW_CHOICES = [
        ("merge", "Merge"),
        ("pr", "Pull Request"),
        ("mr", "Merge Request"),
        ("trunk_based", "Trunk-based"),
    ]

    project = models.ForeignKey(
        Project, on_delete=models.CASCADE, db_column="project_id", related_name="repositories"
    )
    name = models.CharField(max_length=50)
    url = models.CharField(max_length=500)
    branch_default = models.CharField(max_length=100, blank=True, default="")
    kind = models.CharField(max_length=40, blank=True, default="", choices=KIND_CHOICES)
    is_default = models.BooleanField(default=False)
    workflow = models.CharField(max_length=40, blank=True, default="", choices=WORKFLOW_CHOICES)
    notes = models.TextField(blank=True, default="")
    # root_path: carpeta del checkout donde vive este repo (ej. "/" o
    # "/domain/services/"). Opcional. Migración 000159 en domain-mcp.
    root_path = models.CharField(max_length=500, blank=True, default="")

    class Meta:
        db_table = "project_repositories"
        managed = False
        ordering = ["-is_default", "name"]

    def __str__(self) -> str:
        return f"{self.name} → {self.url}"

    @property
    def is_active(self) -> bool:
        return self.deleted_at is None
