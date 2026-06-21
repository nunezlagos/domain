"""Helpers para crear filas reales en la DB de test.

Los PKs son UUID sin default efectivo en test (los genera domain-mcp en prod),
así que se pasan explícitamente. Las API keys cuelgan de un user real.
"""
from __future__ import annotations

import hashlib
import uuid

from users.models import User

from apikeys.models import ApiKey


def make_user(email: str = "owner@example.com", *, status: str = "active") -> User:
    return User.objects.create(
        id=uuid.uuid4(),
        email=email,
        name=email.split("@")[0],
        role="viewer",
        status=status,
    )


def make_api_key(
    name: str = "CI Pipeline",
    *,
    user: User | None = None,
    status: str = "active",
    key_prefix: str = "sk_test1234",
    revoked: bool = False,
    expires_at=None,
) -> ApiKey:
    if user is None:
        user = make_user(f"{uuid.uuid4().hex[:8]}@example.com")
    key_hash = hashlib.sha256(name.encode("utf-8")).digest()
    ak = ApiKey.objects.create(
        id=uuid.uuid4(),
        user=user,
        name=name,
        key_prefix=key_prefix,
        key_hash=key_hash,
        status=status,
        expires_at=expires_at,
    )
    if revoked:
        from django.utils import timezone
        ak.revoked_at = timezone.now()
        ak.status = "revoked"
        ak.save()
    return ak
