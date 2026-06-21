"""HU-API.1: Modelo del mantenedor de API Keys.

Tabla existente en domain-mcp:
- auth_api_keys: credenciales de API (bcrypt-hashed) con prefix visible.
  (creada como `api_keys` en 000004; renombrada a `auth_api_keys` en 000154)

Django NO migra esta tabla (managed=False). Solo lee/escribe via ORM.

Schema real (information_schema de la BD viva):
    id               uuid PK        DEFAULT gen_random_uuid()
    user_id          uuid NOT NULL  FK -> users.id ON DELETE CASCADE
    key_hash         bytea NOT NULL
    key_prefix       varchar(20) NOT NULL
    name             varchar(255) NOT NULL
    last_used_at     timestamptz    NULLABLE
    expires_at       timestamptz    NULLABLE
    revoked_at       timestamptz    NULLABLE  (soft-delete)
    created_at       timestamptz NOT NULL DEFAULT NOW()
    updated_at       timestamptz NOT NULL DEFAULT NOW()  (trigger set_updated_at)
    status           text NOT NULL  DEFAULT 'active'

NOTA: organization_id fue dropeada (al eliminar la tabla organizations).
NO existe en la tabla real; NO se declara acá.

Soft-delete: revocar setea revoked_at + status='revoked' (NO borra fila).
"""
import uuid

from django.db import models

from users.models import User


class ApiKey(models.Model):
    """API Key de la plataforma. PK uuid.

    El secreto en claro NUNCA se almacena ni se reconstruye desde el admin:
    solo se ve `key_prefix` (los primeros chars) y el `key_hash` (bcrypt).
    """

    STATUS_CHOICES = [
        ("active", "Activa"),
        ("revoked", "Revocada"),
        ("expired", "Expirada"),
    ]

    id = models.UUIDField(primary_key=True, default=uuid.uuid4)
    user = models.ForeignKey(
        User,
        on_delete=models.CASCADE,
        db_column="user_id",
        related_name="api_keys",
    )
    key_hash = models.BinaryField()
    key_prefix = models.CharField(max_length=20)
    name = models.CharField(max_length=255)
    last_used_at = models.DateTimeField(null=True, blank=True)
    expires_at = models.DateTimeField(null=True, blank=True)
    revoked_at = models.DateTimeField(null=True, blank=True)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
    status = models.CharField(max_length=20, default="active", choices=STATUS_CHOICES)

    class Meta:
        db_table = "auth_api_keys"
        managed = False
        ordering = ["-created_at"]

    def __str__(self) -> str:
        return f"{self.name} ({self.key_prefix}…)"

    @property
    def display_name(self) -> str:
        return self.name or self.key_prefix

    @property
    def is_active(self) -> bool:
        """Activa = status active, sin revocar y sin expirar."""
        from django.utils import timezone

        if self.status != "active" or self.revoked_at is not None:
            return False
        if self.expires_at is not None and self.expires_at <= timezone.now():
            return False
        return True

    @property
    def is_expired(self) -> bool:
        from django.utils import timezone

        return self.expires_at is not None and self.expires_at <= timezone.now()
