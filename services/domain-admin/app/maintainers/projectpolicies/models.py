"""Modelo del mantenedor de Reglas por proyecto (project_policies).

Tabla existente en domain-mcp (migracion 000106_create_project_policies),
managed=False: Django solo lee/escribe via ORM; el PK uuid lo genera domain-mcp.

ProjectPolicy hereda de core.models.SoftDeleteModel porque la tabla tiene
deleted_at + status. Solo se declaran las columnas PROPIAS. Deben matchear
EXACTO la tabla real (guard: core/tests/test_schema_drift.py).

NOTA: organization_id y las RLS se dropearon (sistema org-less, mig 000142);
por eso el modelo NO declara organization_id. body_structured es JSONB.
"""
from __future__ import annotations

from django.db import models

from core.models import SoftDeleteModel


class ProjectPolicy(SoftDeleteModel):
    """Regla (policy) scopeada a un proyecto.

    id / created_at / updated_at / deleted_at / status vienen de SoftDeleteModel.
    is_active controla la habilitacion; override_platform define si reemplaza
    (true) o amplia (false) la regla de plataforma del mismo kind.
    """

    KIND_CHOICES = [
        ("convention", "Convencion"),
        ("security_rule", "Regla de seguridad"),
        ("architecture", "Arquitectura"),
        ("sdd_workflow", "Workflow SDD"),
        ("observability", "Observabilidad"),
        ("migration_rule", "Regla de migracion"),
        ("linter_config", "Config de linter"),
        ("agent_protocol", "Protocolo de agente"),
        ("git_workflow", "Workflow git"),
        ("tech_stack", "Stack tecnico"),
        ("test_strategy", "Estrategia de tests"),
    ]
    SOURCE_CHOICES = [
        ("manual", "Manual"),
        ("llm_generated", "Generada por LLM"),
        ("seed_imported", "Importada de seed"),
        ("dashboard", "Dashboard"),
    ]

    project_id = models.UUIDField()
    slug = models.CharField(max_length=80)
    name = models.CharField(max_length=160)
    kind = models.CharField(max_length=40, choices=KIND_CHOICES)
    body_md = models.TextField()
    body_structured = models.JSONField(default=dict, blank=True)
    version = models.IntegerField(default=1)
    is_active = models.BooleanField(default=True)
    override_platform = models.BooleanField(default=False)
    source = models.CharField(max_length=40, default="dashboard", choices=SOURCE_CHOICES)
    proposed = models.BooleanField(default=False)

    class Meta:
        db_table = "project_policies"
        managed = False
        ordering = ["-created_at"]

    def __str__(self) -> str:
        return f"{self.name} ({self.slug})"

    @property
    def display_name(self) -> str:
        return self.name or self.slug

    @property
    def is_enabled(self) -> bool:
        return self.is_active and self.deleted_at is None
