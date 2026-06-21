"""HU-48.1: Modelos del mantenedor de usuarios.

3 tablas existentes en domain-mcp:
- users: usuarios de la plataforma (managed=False)
- roles: roles fijos/seeded (managed=False, solo lectura)
- user_roles: pivote many-to-many (managed=False)

Django NO migra estas tablas (managed=False). Solo lee/escribe via ORM.
"""
import uuid

from django.contrib.postgres.fields import ArrayField
from django.db import models


class Role(models.Model):
    """Rol de la plataforma. Tabla seeded, no se crea/edita desde admin."""

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


class User(models.Model):
    """Usuario de la plataforma. PK uuid."""

    STATUS_CHOICES = [
        ("active", "Activo"),
        ("pending", "Pendiente"),
        ("suspended", "Suspendido"),
        ("revoked", "Revocado"),
    ]

    id = models.UUIDField(primary_key=True, default=uuid.uuid4)
    email = models.EmailField(unique=True)
    name = models.CharField(max_length=200, blank=True, default="")
    role = models.CharField(max_length=50, default="viewer")  # rol "principal"
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
    deleted_at = models.DateTimeField(null=True, blank=True)
    rut = models.CharField(max_length=20, blank=True, default="")
    last_organization_id = models.UUIDField(null=True, blank=True)
    last_login_at = models.DateTimeField(null=True, blank=True)
    is_erased = models.BooleanField(default=False)
    erased_at = models.DateTimeField(null=True, blank=True)
    password_hash = models.BinaryField(null=True, blank=True)
    password_set_at = models.DateTimeField(null=True, blank=True)
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

    Schema:
        id          uuid PK
        user_id     uuid FK -> users.id
        role_id     uuid FK -> roles.id
        granted_at  timestamptz
        granted_by  uuid (nullable)
    """

    id = models.UUIDField(primary_key=True, default=uuid.uuid4)
    user = models.ForeignKey(User, on_delete=models.CASCADE, db_column="user_id", related_name="roles")
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