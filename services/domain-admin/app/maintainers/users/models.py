"""Modelos del mantenedor de usuarios (migrado a core).

2 tablas existentes en domain-mcp (managed=False, Django solo lee/escribe):
- users:      usuarios de la plataforma  -> hereda de core.SoftDeleteModel
- roles:      roles fijos/seeded          -> model propio (alimenta el dropdown del form)

User reusa los campos comunes (id/created_at/updated_at/deleted_at/status) de
core.models.SoftDeleteModel y declara SOLO sus columnas propias. Las columnas
declaradas deben matchear EXACTO la tabla real `users` (guard:
core/tests/test_schema_drift.py).
"""
from __future__ import annotations

import uuid

from django.contrib.postgres.fields import ArrayField
from django.db import models

from core.models import SoftDeleteModel


class Role(models.Model):
    """Rol de la plataforma. Tabla seeded, no se crea/edita desde admin.

    NO migrado a core a propósito: el área de roles/permisos quedó reservada
    (la maneja Django / pedido del usuario). Se mueve tal cual estaba.
    """

    id = models.UUIDField(primary_key=True, default=uuid.uuid4)
    slug = models.CharField(max_length=50, unique=True)
    name = models.CharField(max_length=100)
    description = models.TextField(blank=True, default="")
    # En la DB es text[] (no jsonb), por eso ArrayField en lugar de JSONField.
    permissions = ArrayField(models.CharField(max_length=100), default=list, blank=True)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
    status = models.CharField(max_length=20, default="active")

    class Meta:
        db_table = "roles"
        managed = False
        ordering = ["slug"]

    def __str__(self) -> str:
        return f"{self.name} ({self.slug})"


class User(SoftDeleteModel):
    """Usuario de la plataforma.

    id / created_at / updated_at / deleted_at / status vienen de SoftDeleteModel.
    `status` se redeclara solo para sumarle choices (misma columna).
    """

    STATUS_CHOICES = [
        ("active", "Activo"),
        ("pending", "Pendiente"),
        ("suspended", "Suspendido"),
        ("revoked", "Revocado"),
    ]

    email = models.EmailField(unique=True)
    name = models.CharField(max_length=200, blank=True, default="")
    role = models.CharField(max_length=50, default="viewer")  # rol "principal"
    rut = models.CharField(max_length=20, blank=True, default="")
    last_organization_id = models.UUIDField(null=True, blank=True)
    last_login_at = models.DateTimeField(null=True, blank=True)
    is_erased = models.BooleanField(default=False)
    erased_at = models.DateTimeField(null=True, blank=True)
    password_hash = models.BinaryField(null=True, blank=True)
    password_set_at = models.DateTimeField(null=True, blank=True)
    # Redeclara status (heredado de SoftDeleteModel) solo para agregar choices.
    status = models.CharField(max_length=20, default="active", choices=STATUS_CHOICES)

    class Meta:
        db_table = "users"
        managed = False
        ordering = ["-created_at"]

    def __str__(self) -> str:
        return self.email

    @property
    def display_name(self) -> str:
        return self.name or self.email

    @property
    def is_active(self) -> bool:
        return self.status == "active" and not self.is_erased and self.deleted_at is None
