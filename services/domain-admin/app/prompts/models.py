"""Modelos del mantenedor de Prompts.

Tabla existente en domain-mcp (migración 000008_create_prompts):
- prompts: prompts versionados con variables tipadas, opcionalmente
  aislados por project_id. La columna organization_id fue eliminada al
  borrar la tabla organizations (Fase C), así que NO se mapea.
  Soft-delete vía deleted_at. Toggle de habilitación vía is_active (bool).
  status (free-text, default 'active') estandarizado en 000120.

Django NO migra esta tabla (managed=False). Solo lee/escribe vía ORM;
las filas (incluido el PK uuid) las genera domain-mcp en producción.
"""
import uuid

from django.contrib.postgres.fields import ArrayField
from django.db import models


class Prompt(models.Model):
    """Prompt versionado. PK uuid.

    Schema real (prompts):
        id                uuid PK default gen_random_uuid()
        project_id        uuid NULL FK projects(id)
        created_by        uuid NULL FK users(id) ON DELETE SET NULL
        slug              varchar(100) NOT NULL
        version           int NOT NULL default 1
        body              text NOT NULL
        body_tsv          tsvector GENERATED (NO se mapea: columna generada)
        variables         jsonb NOT NULL default '[]'
        description       text NULL
        is_active         boolean NOT NULL default true
        parent_version_id uuid NULL
        tags              text[] NOT NULL default '{}'
        created_at        timestamptz NOT NULL default now()
        updated_at        timestamptz NOT NULL default now()  (trigger set_updated_at)
        deleted_at        timestamptz NULL
        status            text NOT NULL default 'active'
        UNIQUE (project_id, slug, version)

    Nota: body_tsv es una columna GENERATED ALWAYS STORED; no se mapea como
    campo del modelo (Django no la escribe y Postgres la calcula sola).
    organization_id ya NO existe (se dropeó con la tabla organizations).
    """

    id = models.UUIDField(primary_key=True, default=uuid.uuid4)
    project_id = models.UUIDField(null=True, blank=True)
    created_by = models.UUIDField(null=True, blank=True)
    slug = models.CharField(max_length=100)
    version = models.IntegerField(default=1)
    body = models.TextField()
    variables = models.JSONField(default=list, blank=True)
    description = models.TextField(blank=True, default="")
    is_active = models.BooleanField(default=True)
    parent_version_id = models.UUIDField(null=True, blank=True)
    tags = ArrayField(
        models.CharField(max_length=100), default=list, blank=True
    )
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
    deleted_at = models.DateTimeField(null=True, blank=True)
    status = models.CharField(max_length=20, default="active")

    class Meta:
        db_table = "prompts"
        managed = False
        ordering = ["-created_at"]

    def __str__(self) -> str:
        return f"{self.slug} v{self.version}"

    @property
    def display_name(self) -> str:
        return f"{self.slug} v{self.version}"

    @property
    def is_enabled(self) -> bool:
        """Habilitado = is_active y no soft-deleted.

        Se llama is_enabled (no is_active) para no chocar con la columna
        real `is_active`; el template usa is_enabled para decidir el badge
        y el botón de toggle.
        """
        return self.is_active and self.deleted_at is None
