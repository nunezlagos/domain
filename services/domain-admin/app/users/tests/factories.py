"""Helpers para crear filas reales en la DB de test.

Los PKs son UUID sin default en el modelo (los genera domain-mcp en prod),
así que en tests hay que pasarlos explícitamente.
"""
from __future__ import annotations

import uuid

from users.models import Role, User, UserRole


def make_role(slug: str = "viewer", *, name: str | None = None, status: str = "active",
              permissions: list[str] | None = None) -> Role:
    return Role.objects.create(
        id=uuid.uuid4(),
        slug=slug,
        name=name or slug.capitalize(),
        permissions=permissions or [],
        status=status,
    )


def make_user(email: str, *, name: str = "", role: str = "viewer",
              status: str = "active", deleted: bool = False) -> User:
    u = User.objects.create(
        id=uuid.uuid4(),
        email=email,
        name=name,
        role=role,
        status=status,
    )
    if deleted:
        from django.utils import timezone
        u.deleted_at = timezone.now()
        u.status = "revoked"
        u.save()
    return u


def make_user_role(user: User, role: Role, *, status: str = "active") -> UserRole:
    return UserRole.objects.create(id=uuid.uuid4(), user=user, role=role, status=status)
