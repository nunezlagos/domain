"""Modelos del mantenedor de Proyectos.

3 tablas existentes en domain-mcp (Django NO las migra, managed=False; solo
lee/escribe vía ORM, las filas y el PK uuid los genera domain-mcp en prod):

- projects (mig 000005 + 000087 + 000100 + 000108): proyecto de la
  organización. Soft-delete vía deleted_at (NO tiene columna status; el
  estado activo/archivado es derivado de deleted_at). Entidad principal de
  este mantenedor.
- project_templates (mig 000021): templates preconfigurados. SIN deleted_at,
  SIN status. Catálogo de solo-lectura desde el admin (seeded). Sirve para el
  selector "template" del form de proyecto.
- project_repositories (mig 000105): N remotos git por proyecto. Soft-delete
  vía deleted_at + flag is_default (1 default por proyecto activo).

Convención (igual que el app `clients`): las columnas que referencian tablas
fuera de este app (organizations, clients) se modelan como UUIDField plano,
NO como ForeignKey de Django, para no acoplar el schema de test a tablas de
otros servicios.
"""
import uuid

from django.contrib.postgres.fields import ArrayField
from django.db import models


class ProjectTemplate(models.Model):
    """Template de proyecto. Catálogo seeded, solo-lectura desde el admin.

    Schema real (project_templates):
        id               uuid PK default gen_random_uuid()
        organization_id  uuid NULL FK organizations(id)
        slug             varchar(100) NOT NULL  (unique per organization_id)
        name             varchar(255) NOT NULL
        description      text NULL
        is_default       bool NOT NULL default false
        is_public        bool NOT NULL default false
        settings         jsonb NOT NULL default '{}'
        default_skills   text[] NOT NULL default '{}'
        default_agents   text[] NOT NULL default '{}'
        default_flows    text[] NOT NULL default '{}'
        seed_managed     bool NOT NULL default false
        seed_version     int NULL
        is_user_modified bool NOT NULL default false
        created_at       timestamptz NOT NULL default now()
        updated_at       timestamptz NOT NULL default now()  (trigger set_updated_at)
        UNIQUE (organization_id, slug)
    """

    id = models.UUIDField(primary_key=True, default=uuid.uuid4)
    organization_id = models.UUIDField(null=True, blank=True)
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

    class Meta:
        db_table = "project_templates"
        managed = False
        ordering = ["slug"]

    def __str__(self) -> str:
        return f"{self.name} ({self.slug})"


class Project(models.Model):
    """Proyecto de la organización. PK uuid. Soft-delete vía deleted_at.

    NO tiene columna `status` en la DB: el estado es DERIVADO de deleted_at
    (activo si deleted_at IS NULL, archivado si está seteado). El "toggle"
    archiva/restaura el proyecto.

    Schema real (projects, tras migraciones 000005/000087/000100/000108):
        id               uuid PK default gen_random_uuid()
        organization_id  uuid NOT NULL FK organizations(id)
        name             varchar(255) NOT NULL
        slug             varchar(100) NOT NULL  (unique per organization_id)
        description      text NULL
        repository_url   varchar(500) NULL
        template_id      uuid NULL FK project_templates(id)
        settings         jsonb NOT NULL default '{}'
        current_branch   varchar(120) NULL
        rules            text[] NOT NULL default '{}'
        client_id        uuid NULL FK clients(id)
        last_known_head  varchar(40) NULL
        last_seen_at     timestamptz NULL
        last_seen_branch varchar(120) NULL
        last_seen_cwd    varchar(500) NULL
        created_at       timestamptz NOT NULL default now()
        updated_at       timestamptz NOT NULL default now()  (trigger set_updated_at)
        deleted_at       timestamptz NULL
        UNIQUE (organization_id, slug)
    """

    # Estado DERIVADO (no es columna): para el badge y get_status_display.
    STATUS_ACTIVE = "active"
    STATUS_ARCHIVED = "archived"
    STATUS_CHOICES = [
        (STATUS_ACTIVE, "Activo"),
        (STATUS_ARCHIVED, "Archivado"),
    ]

    id = models.UUIDField(primary_key=True, default=uuid.uuid4)
    organization_id = models.UUIDField()
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
    def status(self) -> str:
        """Estado derivado de deleted_at (no es columna)."""
        return self.STATUS_ARCHIVED if self.deleted_at is not None else self.STATUS_ACTIVE

    @property
    def is_active(self) -> bool:
        return self.deleted_at is None

    def get_status_display(self) -> str:
        return dict(self.STATUS_CHOICES).get(self.status, self.status)


class ProjectRepository(models.Model):
    """Remoto git de un proyecto. Soft-delete vía deleted_at + flag is_default.

    Schema real (project_repositories):
        id               uuid PK default gen_random_uuid()
        organization_id  uuid NOT NULL FK organizations(id)
        project_id       uuid NOT NULL FK projects(id)
        name             varchar(50) NOT NULL  (alias del remoto; CHECK <> '')
        url              varchar(500) NOT NULL (CHECK <> '')
        branch_default   varchar(100) NULL
        kind             varchar(40) NULL  (github|gitlab|bitbucket|gitea|other)
        is_default       bool NOT NULL default false
        workflow         varchar(40) NULL  (merge|pr|mr|trunk_based)
        notes            text NULL
        created_at       timestamptz NOT NULL default now()
        updated_at       timestamptz NOT NULL default now()  (trigger set_updated_at)
        deleted_at       timestamptz NULL
        UNIQUE (organization_id, project_id, name)
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
    organization_id = models.UUIDField()
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

    class Meta:
        db_table = "project_repositories"
        managed = False
        ordering = ["-is_default", "name"]

    def __str__(self) -> str:
        return f"{self.name} → {self.url}"

    @property
    def is_active(self) -> bool:
        return self.deleted_at is None
