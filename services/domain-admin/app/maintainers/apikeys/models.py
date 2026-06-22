"""Modelo del mantenedor de API Keys (migrado a core).

Tabla existente en domain-mcp (managed=False, Django solo lee/escribe):
- auth_api_keys: credenciales de API (bcrypt-hashed) con prefix visible y, desde
  la mig 000164, la key en claro (key_plaintext) para poder re-mostrarla.
  (creada como `api_keys` en 000004; renombrada a `auth_api_keys` en 000154)

ApiKey reusa id/created_at/updated_at de core.models.BaseModel y declara SOLO
sus columnas propias. NO hereda de SoftDeleteModel: el soft-delete de esta
tabla es `revoked_at` (NO `deleted_at`, que la tabla real NO tiene). Declarar
`deleted_at` romperia el guard de schema drift (core/tests/test_schema_drift.py).

Columnas reales (core/tests/real_schema.json -> auth_api_keys):
    id, user_id, key_hash, key_prefix, key_plaintext, name, last_used_at,
    expires_at, revoked_at, created_at, updated_at, status

NOTA: organization_id fue dropeada; NO existe en la tabla real, NO se declara.
"""
from __future__ import annotations

from django.db import models

from core.models import BaseModel
from maintainers.users.models import User


class ApiKey(BaseModel):
    """API Key de la plataforma. PK uuid (de BaseModel).

    Formato `domk_<env>_<secret>` + `key_hash` = bcrypt (lo que valida el MCP).
    `key_plaintext` guarda la key en claro para poder re-mostrarla en el detalle
    (decision de producto: el owner quiere verla de nuevo). TRADEOFF: la key
    queda recuperable; endurecer a futuro con cifrado at-rest.

    id / created_at / updated_at vienen de BaseModel. `status` y `revoked_at`
    se declaran aqui (el soft-delete de esta tabla es revoked_at, no deleted_at).
    """

    STATUS_CHOICES = [
        ("active", "Activa"),
        ("revoked", "Revocada"),
        ("expired", "Expirada"),
    ]

    user = models.ForeignKey(
        User,
        on_delete=models.CASCADE,
        db_column="user_id",
        related_name="api_keys",
    )
    key_hash = models.BinaryField()
    key_prefix = models.CharField(max_length=20)
    # key_plaintext: la key en claro, guardada para poder mostrarla de nuevo en
    # el detalle (decision de producto, mig 000164). Nullable: keys viejas
    # creadas antes de esta columna no la tienen.
    key_plaintext = models.TextField(null=True, blank=True)
    name = models.CharField(max_length=255)
    last_used_at = models.DateTimeField(null=True, blank=True)
    expires_at = models.DateTimeField(null=True, blank=True)
    revoked_at = models.DateTimeField(null=True, blank=True)
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
