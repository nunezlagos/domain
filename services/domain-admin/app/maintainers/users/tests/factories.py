"""Factories del mantenedor de usuarios.

Reusa core.tests.factories.make (pone un PK uuid si no se pasa `id`, ya que en
prod los genera domain-mcp). Solo agrega los helpers específicos de las
tablas del dominio (users, roles).
"""
from __future__ import annotations

from core.tests.factories import make

from maintainers.users.models import Role, User


def make_role(slug: str = "viewer", *, name: str | None = None, status: str = "active",
              permissions: list[str] | None = None) -> Role:
    return make(
        Role,
        slug=slug,
        name=name or slug.capitalize(),
        permissions=permissions or [],
        status=status,
    )


def make_user(email: str, *, name: str = "", role: str = "viewer",
              status: str = "active", deleted: bool = False) -> User:
    u = make(User, email=email, name=name, role=role, status=status)
    if deleted:
        from django.utils import timezone
        u.deleted_at = timezone.now()
        u.status = "revoked"
        u.save()
    return u
