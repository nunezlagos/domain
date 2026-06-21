"""Modelos del mantenedor de usuarios (migrado a core).

3 tablas existentes en domain-mcp (managed=False, Django solo lee/escribe):
- users:      usuarios de la plataforma  -> hereda de core.SoftDeleteModel
- roles:      roles fijos/seeded          -> model propio (NO se toca el área roles)
- user_roles: pivote many-to-many         -> model propio (PK compuesta, excluido del guard)

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


class UserRole(models.Model):
    """Pivote: asigna roles a users (many-to-many con metadata).

    La tabla real user_roles tiene PK COMPUESTA (user_id, role_id) y NO tiene
    columna `id`. Django 5.1 no soporta CompositePrimaryKey, así que usamos
    `user` como primary_key (db_column user_id). Alcanza para lo que hace el app:
    leer (filter por user/role), asignar (create) y revocar (delete por filter).
    El INSERT no manda `id` (no existe) y la unicidad real (user_id, role_id) la
    impone la BD.

    Schema real:
        user_id     uuid  (FK -> users.id)
        role_id     uuid  (FK -> roles.id)
        granted_at / granted_by / created_at / updated_at / status
    """

    user = models.ForeignKey(User, on_delete=models.CASCADE, db_column="user_id", related_name="roles", primary_key=True)
    role = models.ForeignKey(Role, on_delete=models.CASCADE, db_column="role_id", related_name="users")
    granted_at = models.DateTimeField(auto_now_add=True)
    granted_by = models.UUIDField(null=True, blank=True)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
    status = models.CharField(max_length=20, default="active")

    class Meta:
        db_table = "user_roles"
        managed = False
        unique_together = [("user", "role")]

    def __str__(self) -> str:
        return f"{self.user.email} → {self.role.slug}"
