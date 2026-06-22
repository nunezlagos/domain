"""Modelos del mantenedor de Flows (migrado a core).

2 tablas existentes en domain-mcp (managed=False, Django solo lee/escribe):
- flows:         DAGs declarativos con spec JSONB  -> hereda de core.SoftDeleteModel
- flow_versions: snapshots inmutables por version  -> hereda de core.BaseModel (READ-ONLY)

Flow reusa los campos comunes (id/created_at/updated_at/deleted_at/status) de
core.models.SoftDeleteModel y declara SOLO sus columnas propias. FlowVersion
reusa id/created_at/updated_at de core.models.BaseModel (no tiene deleted_at).
Las columnas declaradas deben matchear EXACTO las tablas reales (guard:
core/tests/test_schema_drift.py + core/tests/real_schema.json).

El estado habilitado/deshabilitado del flow es el boolean `is_active`; el
toggle alterna ese boolean. La baja es soft (deleted_at) y ademas deshabilita
(is_active=false). NO existe `organization_id` en la tabla real.
"""
from __future__ import annotations

from django.db import models

from core.models import BaseModel, SoftDeleteModel


class Flow(SoftDeleteModel):
    """Flow (DAG declarativo).

    id / created_at / updated_at / deleted_at / status vienen de SoftDeleteModel.
    Soft-delete via deleted_at; habilitado/deshabilitado via is_active (boolean).

    Schema real (flows):
        id                   uuid PK
        slug                 varchar(100) NOT NULL
        name                 varchar(255) NOT NULL
        description          text NULL
        spec                 jsonb NOT NULL
        is_active            boolean NOT NULL default true
        deterministic_replay boolean NOT NULL default false
        seed_managed         boolean NOT NULL default false
        seed_version         int NULL
        is_user_modified     boolean NOT NULL default false
        created_at           timestamptz NOT NULL
        updated_at           timestamptz NOT NULL  (trigger set_updated_at)
        deleted_at           timestamptz NULL
        status               varchar
    """

    slug = models.CharField(max_length=100)
    name = models.CharField(max_length=255)
    description = models.TextField(blank=True, default="")
    spec = models.JSONField(default=dict, blank=True)
    is_active = models.BooleanField(default=True)
    deterministic_replay = models.BooleanField(default=False)
    seed_managed = models.BooleanField(default=False)
    seed_version = models.IntegerField(null=True, blank=True)
    is_user_modified = models.BooleanField(default=False)

    class Meta:
        db_table = "flows"
        managed = False
        ordering = ["-created_at"]

    def __str__(self) -> str:
        return f"{self.name} ({self.slug})"

    @property
    def display_name(self) -> str:
        return self.name or self.slug

    @property
    def is_live(self) -> bool:
        """Habilitado y no eliminado. (El campo crudo is_active es el toggle.)"""
        return self.is_active and self.deleted_at is None

    @property
    def status_label(self) -> str:
        """Etiqueta ES derivada del boolean is_active (no del campo status)."""
        if self.deleted_at is not None:
            return "Eliminado"
        return "Activo" if self.is_active else "Inactivo"


class FlowVersion(BaseModel):
    """Snapshot inmutable de la definicion de un flow. READ-ONLY en el admin.

    id / created_at / updated_at vienen de BaseModel. Sin CRUD por modal: se
    muestran como lista read-only en el detalle del flow padre (analogo a
    user_roles en users/).

    Schema real (flow_versions):
        id          uuid PK
        flow_id     uuid NOT NULL FK flows(id) ON DELETE CASCADE
        version     int NOT NULL
        definition  jsonb NOT NULL
        hash        varchar(64) NOT NULL  (SHA-256 hex)
        note        text NULL
        created_by  uuid NULL FK users(id) ON DELETE SET NULL
        created_at  timestamptz NOT NULL
        updated_at  timestamptz NOT NULL
        status      varchar
        is_default  boolean
        published_at timestamptz NULL
        deprecated_at timestamptz NULL
        UNIQUE (flow_id, version)
        UNIQUE (flow_id, hash)
    """

    flow = models.ForeignKey(
        Flow,
        on_delete=models.CASCADE,
        db_column="flow_id",
        related_name="versions",
    )
    version = models.IntegerField()
    definition = models.JSONField(default=dict, blank=True)
    hash = models.CharField(max_length=64)
    note = models.TextField(blank=True, default="")
    created_by = models.UUIDField(null=True, blank=True)
    status = models.CharField(max_length=20, default="active")
    is_default = models.BooleanField(default=False)
    published_at = models.DateTimeField(null=True, blank=True)
    deprecated_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "flow_versions"
        managed = False
        ordering = ["-version"]

    def __str__(self) -> str:
        return f"{self.flow_id} v{self.version}"

    @property
    def short_hash(self) -> str:
        return self.hash[:12] if self.hash else ""
