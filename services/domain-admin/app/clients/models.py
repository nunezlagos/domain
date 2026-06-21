"""Modelos del mantenedor de Clientes (mandantes).

Tabla existente en domain-mcp (migración 000099_create_clients):
- clients: cuentas/empresas externas que la organización gestiona como
  contraparte (clientes finales, partners, contratantes). Aislada por
  organization_id (multi-tenant). Soft-delete vía deleted_at + status.

Django NO migra esta tabla (managed=False). Solo lee/escribe vía ORM;
las filas (incluido el PK uuid) las genera domain-mcp en producción.
"""
import uuid

from django.db import models


class Client(models.Model):
    """Cliente/mandante de la plataforma. PK uuid.

    Schema real (clients):
        id              uuid PK default gen_random_uuid()
        organization_id uuid NOT NULL FK organizations(id)
        name            varchar(255) NOT NULL
        slug            varchar(100) NOT NULL  (unique per organization_id)
        tax_id          varchar(50) NULL
        contact_email   varchar(255) NULL
        contact_phone   varchar(50) NULL
        address         text NULL
        metadata        jsonb NOT NULL default '{}'
        status          varchar(20) NOT NULL default 'active'
                        CHECK status IN ('active','inactive','archived')
        created_at      timestamptz NOT NULL default now()
        updated_at      timestamptz NOT NULL default now()  (trigger set_updated_at)
        deleted_at      timestamptz NULL
        UNIQUE (organization_id, slug)
    """

    STATUS_CHOICES = [
        ("active", "Activo"),
        ("inactive", "Inactivo"),
        ("archived", "Archivado"),
    ]

    id = models.UUIDField(primary_key=True, default=uuid.uuid4)
    organization_id = models.UUIDField()
    name = models.CharField(max_length=255)
    slug = models.CharField(max_length=100)
    tax_id = models.CharField(max_length=50, blank=True, default="")
    contact_email = models.CharField(max_length=255, blank=True, default="")
    contact_phone = models.CharField(max_length=50, blank=True, default="")
    address = models.TextField(blank=True, default="")
    metadata = models.JSONField(default=dict, blank=True)
    status = models.CharField(max_length=20, default="active", choices=STATUS_CHOICES)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
    deleted_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "clients"
        managed = False
        ordering = ["-created_at"]

    def __str__(self) -> str:
        return f"{self.name} ({self.slug})"

    @property
    def display_name(self) -> str:
        return self.name or self.slug

    @property
    def is_active(self) -> bool:
        return self.status == "active" and self.deleted_at is None
