"""Modelos del mantenedor de Proyectos.

3 tablas existentes en domain-mcp (Django NO las migra, managed=False; solo
lee/escribe vía ORM, las filas y el PK uuid los genera domain-mcp en prod):

- projects: proyecto. Soft-delete vía deleted_at + columna status. Entidad
  principal de este mantenedor.
- project_templates: templates preconfigurados. SIN deleted_at (solo status).
  Catálogo de solo-lectura desde el admin (seeded). Sirve para el selector
  "template" del form de proyecto.
- project_repositories: N remotos git por proyecto. Soft-delete vía deleted_at
  + columna status + flag is_default (1 default por proyecto activo).

NOTA: la tabla `organizations` fue eliminada y la columna `organization_id`
fue dropeada de estas tablas. Por eso NINGÚN modelo declara organization_id.
Las columnas declaradas matchean EXACTO el schema real (information_schema).
"""
import uuid

from django.contrib.postgres.fields import ArrayField
from django.db import models


class ProjectTemplate(models.Model):
    """Template de proyecto. Catálogo seeded, solo-lectura desde el admin.

    Columnas reales (project_templates): id, slug, name, description,
    is_default, is_public, settings, default_skills, default_agents,
    default_flows, seed_managed, seed_version, is_user_modified, created_at,
    updated_at, status. SIN deleted_at.
    """

    id = models.UUIDField(primary_key=True, default=uuid.uuid4)
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
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
    status = models.CharField(max_length=20, default="active")

    class Meta:
        db_table = "project_templates"
        managed = False
        ordering = ["slug"]

    def __str__(self) -> str:
        return f"{self.name} ({self.slug})"


class Project(models.Model):
    """Proyecto. PK uuid. Soft-delete vía deleted_at + columna status.

    Columnas reales (projects): id, name, slug, description, repository_url,
    template_id, settings, created_at, updated_at, deleted_at, current_branch,
    rules, client_id, last_known_head, last_seen_at, last_seen_branch,
    last_seen_cwd, status.
    """

    STATUS_ACTIVE = "active"
    STATUS_ARCHIVED = "archived"
    STATUS_CHOICES = [
        (STATUS_ACTIVE, "Activo"),
        (STATUS_ARCHIVED, "Archivado"),
    ]

    id = models.UUIDField(primary_key=True, default=uuid.uuid4)
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
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
    deleted_at = models.DateTimeField(null=True, blank=True)
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


class ProjectRepository(models.Model):
    """Remoto git de un proyecto. Soft-delete vía deleted_at + flag is_default.

    Columnas reales (project_repositories): id, project_id, name,
    branch_default, kind, is_default, workflow, notes, created_at, updated_at,
    deleted_at, status, url.
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

    id = models.UUIDField(primary_key=True, default=uuid.uuid4)
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
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
    deleted_at = models.DateTimeField(null=True, blank=True)
    status = models.CharField(max_length=20, default="active")

    class Meta:
        db_table = "project_repositories"
        managed = False
        ordering = ["-is_default", "name"]

    def __str__(self) -> str:
        return f"{self.name} → {self.url}"

    @property
    def is_active(self) -> bool:
        return self.deleted_at is None
