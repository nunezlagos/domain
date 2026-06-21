"""HU-48.1: services (capa de negocio, separada de views).

Patrón: las views solo hacen HTTP request/response; toda la lógica
de modelo vive acá. Esto facilita testing unitario sin tocar HTTP.
"""
from __future__ import annotations

from typing import Iterable

from django.db import transaction

from .models import Role, User, UserRole


# Errores de dominio (la view los traduce a messages.error).
class UserError(Exception):
    """Error de operación sobre usuarios."""


def list_users(search: str = "", page: int = 1, per_page: int = 20) -> dict:
    """Lista usuarios con búsqueda opcional + paginación.

    Retorna dict con: users, total, page, per_page, total_pages, has_next, has_prev.
    """
    qs = User.objects.all()
    if search:
        qs = qs.filter(email__icontains=search) | qs.filter(name__icontains=search)
    qs = qs.distinct().order_by("-created_at")

    total = qs.count()
    total_pages = max(1, (total + per_page - 1) // per_page)
    start = (page - 1) * per_page
    end = start + per_page
    users = list(qs[start:end])

    return {
        "users": users,
        "total": total,
        "page": page,
        "per_page": per_page,
        "total_pages": total_pages,
        "has_next": end < total,
        "has_prev": page > 1,
    }


def get_user(user_id: str) -> User:
    try:
        return User.objects.get(pk=user_id)
    except User.DoesNotExist as exc:
        raise UserError(f"Usuario {user_id} no existe.") from exc


def get_user_roles(user: User) -> list[UserRole]:
    return list(
        UserRole.objects.filter(user=user, status="active")
        .select_related("role")
        .order_by("role__slug")
    )


def list_available_roles() -> list[Role]:
    return list(Role.objects.filter(status="active").order_by("slug"))


@transaction.atomic
def create_user(
    *,
    email: str,
    name: str,
    role_slug: str,
    status: str,
    hashed_password: bytes | None,
) -> User:
    """Crea un user nuevo. hashed_password debe venir de UserForm.hashed_password()."""
    if User.objects.filter(email__iexact=email).exists():
        raise UserError(f"Ya existe un usuario con email {email}.")

    # Validar que el rol existe.
    if not Role.objects.filter(slug=role_slug, status="active").exists():
        raise UserError(f"Rol '{role_slug}' no existe o no está activo.")

    user = User.objects.create(
        email=email,
        name=name or "",
        role=role_slug,
        status=status,
        password_hash=hashed_password,
    )
    return user


@transaction.atomic
def update_user(
    user: User,
    *,
    email: str,
    name: str,
    role_slug: str,
    status: str,
    hashed_password: bytes | None,
) -> User:
    """Actualiza user. hashed_password=None deja el actual."""
    # Si cambia el email, validar que no choque con otro user.
    if email != user.email and User.objects.filter(email__iexact=email).exclude(pk=user.pk).exists():
        raise UserError(f"Ya existe otro usuario con email {email}.")

    if not Role.objects.filter(slug=role_slug, status="active").exists():
        raise UserError(f"Rol '{role_slug}' no existe o no está activo.")

    user.email = email
    user.name = name or ""
    user.role = role_slug
    user.status = status
    if hashed_password is not None:
        user.password_hash = hashed_password
    user.save()
    return user


@transaction.atomic
def delete_user(user: User) -> None:
    """Soft delete: marca deleted_at. NO borra físicamente."""
    from django.utils import timezone
    user.deleted_at = timezone.now()
    user.status = "revoked"
    user.save()


@transaction.atomic
def assign_role(user: User, role_id: str, granted_by: str | None = None) -> UserRole:
    """Asigna un rol al user. Idempotente (no duplica)."""
    role = Role.objects.get(pk=role_id)
    existing = UserRole.objects.filter(user=user, role=role).first()
    if existing:
        if existing.status != "active":
            existing.status = "active"
            existing.save()
        return existing
    return UserRole.objects.create(user=user, role=role, granted_by=granted_by)


@transaction.atomic
def revoke_role(user: User, role_id: str) -> bool:
    """Revoca un rol del user. Retorna True si se eliminó."""
    qs = UserRole.objects.filter(user=user, role_id=role_id)
    deleted_count, _ = qs.delete()
    return deleted_count > 0


def get_stats() -> dict:
    """Stats agregadas para el header del listado."""
    total = User.objects.count()
    active = User.objects.filter(status="active", deleted_at__isnull=True, is_erased=False).count()
    pending = User.objects.filter(status="pending").count()
    return {"total": total, "active": active, "pending": pending}


@transaction.atomic
def toggle_user_status(user: User) -> str:
    """Alterna active ↔ suspended. Retorna el nuevo status."""
    from django.utils import timezone

    if user.status == "active":
        user.status = "suspended"
    elif user.status == "suspended":
        user.status = "active"
    elif user.status in ("pending", "revoked"):
        user.status = "active"
    else:
        user.status = "suspended"
    user.save()
    return user.status