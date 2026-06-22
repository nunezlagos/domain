"""Factories del mantenedor de API Keys.

Reusa core.tests.factories.make (pone un PK uuid si no se pasa `id`, ya que en
prod los genera domain-mcp) y el make_user del mantenedor de usuarios (las keys
cuelgan de un user real). Solo agrega el helper especifico de auth_api_keys.
"""
from __future__ import annotations

import hashlib
import uuid

from core.tests.factories import make
from maintainers.users.models import User
from maintainers.users.tests.factories import make_user

from maintainers.apikeys.models import ApiKey


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
    ak = make(
        ApiKey,
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
