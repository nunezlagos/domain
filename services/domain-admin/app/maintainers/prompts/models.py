"""Modelo del mantenedor de Prompts (migrado a core).

Tabla existente en domain-mcp (migración 000008_create_prompts), managed=False:
Django solo lee/escribe vía ORM; las filas (incluido el PK uuid) las genera
domain-mcp en producción.

Prompt hereda de core.models.SoftDeleteModel porque la tabla tiene deleted_at +
status: id / created_at / updated_at / deleted_at / status vienen del abstracto
y NO se repiten acá. Solo se declaran las columnas PROPIAS. Las columnas
declaradas deben matchear EXACTO la tabla real `prompts` (guard:
core/tests/test_schema_drift.py).

NOTA: body_tsv es una columna GENERATED ALWAYS STORED; no se mapea (Django no la
escribe y Postgres la calcula sola). organization_id ya NO existe (se dropeó con
la tabla organizations en Fase C).
"""
from __future__ import annotations

from django.contrib.postgres.fields import ArrayField
from django.db import models

from core.models import SoftDeleteModel


class Prompt(SoftDeleteModel):
    """Prompt versionado.

    id / created_at / updated_at / deleted_at / status vienen de SoftDeleteModel.
    La unicidad real es (project_id, slug, version). Soft-delete vía deleted_at;
    toggle de habilitación vía is_active (bool).
    """

    project_id = models.UUIDField(null=True, blank=True)
    created_by = models.UUIDField(null=True, blank=True)
    slug = models.CharField(max_length=100)
    version = models.IntegerField(default=1)
    body = models.TextField()
    variables = models.JSONField(default=list, blank=True)
    description = models.TextField(blank=True, default="")
    is_active = models.BooleanField(default=True)
    parent_version_id = models.UUIDField(null=True, blank=True)
    tags = ArrayField(models.CharField(max_length=100), default=list, blank=True)

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

        Se llama is_enabled (no is_active) para no chocar con la columna real
        `is_active`; el template usa is_enabled para el badge y el toggle.
        """
        return self.is_active and self.deleted_at is None
