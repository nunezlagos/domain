"""Modelo del mantenedor de Politicas de plataforma.

Tabla existente en domain-mcp (migracion 000045_create_platform_policies),
managed=False: Django solo lee/escribe via ORM; el PK uuid lo genera domain-mcp.

PlatformPolicy hereda de core.models.SoftDeleteModel porque la tabla tiene
status. Solo se declaran las columnas PROPIAS. Deben matchear EXACTO la
tabla real (guard: core/tests/test_schema_drift.py).
"""
from __future__ import annotations

from django.db import models

from core.models import BaseModel


class PlatformPolicy(BaseModel):
    """Politica global de plataforma.

    id / created_at / updated_at vienen de BaseModel.
    is_active controla la habilitacion.
    """

    KIND_CHOICES = [
        ("convention", "Convencion"),
        ("security_rule", "Regla de seguridad"),
        ("architecture", "Arquitectura"),
        ("sdd_workflow", "Workflow SDD"),
        ("observability", "Observabilidad"),
        ("migration_rule", "Regla de migracion"),
        ("linter_config", "Config de linter"),
    ]

    slug = models.CharField(max_length=80, unique=False)
    name = models.CharField(max_length=160)
    kind = models.CharField(max_length=40, choices=KIND_CHOICES)
    body_md = models.TextField()
    body_structured = models.JSONField(default=dict, blank=True)
    version = models.IntegerField(default=1)
    is_active = models.BooleanField(default=True)
    source_file = models.CharField(max_length=120, blank=True, default="")

    class Meta:
        db_table = "platform_policies"
        managed = False
        ordering = ["-created_at"]

    def __str__(self) -> str:
        return f"{self.name} ({self.slug})"

    @property
    def display_name(self) -> str:
        return self.name or self.slug

    @property
    def is_enabled(self) -> bool:
        return self.is_active
