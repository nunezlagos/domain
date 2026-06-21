"""Modelos del mantenedor de Flows.

Tablas existentes en domain-mcp:
- flows: DAGs declarativos con spec JSONB. Soft-delete vía deleted_at;
  habilitado/deshabilitado vía is_active (boolean). Tiene además una columna
  `status` (string) en el schema real. NO tiene `organization_id`.
- flow_versions: snapshots inmutables de la definición del flow por versión.
  READ-ONLY desde el admin (sin CRUD): se listan en el detalle del flow padre.

Django NO migra estas tablas (managed=False). Solo lee/escribe vía ORM;
las filas (incluido el PK uuid) las genera domain-mcp en producción.
"""
import uuid

from django.db import models


class Flow(models.Model):
    """Flow (DAG declarativo). PK uuid.

    Schema real (flows):
        id                   uuid PK default gen_random_uuid()
        slug                 varchar(100) NOT NULL
        name                 varchar(255) NOT NULL
        description          text NULL
        spec                 jsonb NOT NULL
        is_active            boolean NOT NULL default true
        deterministic_replay boolean NOT NULL default false
        seed_managed         boolean NOT NULL default false
        seed_version         int NULL
        is_user_modified     boolean NOT NULL default false
        created_at           timestamptz NOT NULL default now()
        updated_at           timestamptz NOT NULL default now()  (trigger set_updated_at)
        deleted_at           timestamptz NULL
        status               varchar

    El estado habilitado/deshabilitado es el boolean `is_active`; el toggle
    alterna ese boolean. La baja es soft (deleted_at) y además deshabilita
    (is_active=false). NO existe `organization_id` en la tabla real.
    """

    id = models.UUIDField(primary_key=True, default=uuid.uuid4)
    slug = models.CharField(max_length=100)
    name = models.CharField(max_length=255)
    description = models.TextField(blank=True, default="")
    spec = models.JSONField(default=dict, blank=True)
    is_active = models.BooleanField(default=True)
    deterministic_replay = models.BooleanField(default=False)
    seed_managed = models.BooleanField(default=False)
    seed_version = models.IntegerField(null=True, blank=True)
    is_user_modified = models.BooleanField(default=False)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
    deleted_at = models.DateTimeField(null=True, blank=True)
    status = models.CharField(max_length=20, default="active")

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
        """Etiqueta ES derivada del boolean is_active (no hay columna status)."""
        if self.deleted_at is not None:
            return "Eliminado"
        return "Activo" if self.is_active else "Inactivo"


class FlowVersion(models.Model):
    """Snapshot inmutable de la definición de un flow. READ-ONLY en el admin.

    Schema real (flow_versions):
        id          uuid PK default gen_random_uuid()
        flow_id     uuid NOT NULL FK flows(id) ON DELETE CASCADE
        version     int NOT NULL
        definition  jsonb NOT NULL
        hash        varchar(64) NOT NULL  (SHA-256 hex)
        note        text NULL
        created_by  uuid NULL FK users(id) ON DELETE SET NULL
        created_at  timestamptz NOT NULL default now()
        UNIQUE (flow_id, version)
        UNIQUE (flow_id, hash)

    Sin CRUD por modal: se muestran como lista read-only en el detalle del
    flow padre (análogo a user_roles en users/).
    """

    id = models.UUIDField(primary_key=True, default=uuid.uuid4)
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
    created_at = models.DateTimeField(auto_now_add=True)

    class Meta:
        db_table = "flow_versions"
        managed = False
        ordering = ["-version"]

    def __str__(self) -> str:
        return f"{self.flow_id} v{self.version}"

    @property
    def short_hash(self) -> str:
        return self.hash[:12] if self.hash else ""
