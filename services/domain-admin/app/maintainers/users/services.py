"""Capa de negocio del mantenedor de usuarios (migrada a core).

list + signal se delegan a core.service.MaintainerService (sin reimplementar la
búsqueda/paginación ni el aggregate de la señal). El resto —roles, password,
create/update/delete/toggle con sus validaciones de dominio— sigue acá.

Las views (core.views.MaintainerViews) descubren las funciones por convención
de nombre: get_user / create_user / update_user / delete_user /
toggle_user_status / get_list_signal. `entity_label="Usuario"` -> attr "usuario",
por eso además exponemos alias get_usuario/... para el descubrimiento del core.
"""
from __future__ import annotations

from django.db import transaction

from core.service import MaintainerService

from .models import Role, User


# Error de dominio (la view lo traduce a messages.error).
class UserError(Exception):
    """Error de operación sobre usuarios."""


# Service base reusado: list (search email/name + paginación) + signal.
class UserService(MaintainerService):
    model = User
    search_fields = ("email", "name")
    ordering = ("-created_at",)


_service = UserService()


def list_users(search: str = "", page: int = 1, per_page: int = 20) -> dict:
    """Lista usuarios con búsqueda + paginación.

    Delega en MaintainerService.list y renombra la clave `items` -> `users`
    para no romper el contrato del template/tests existentes.
    """
    data = _service.list(search=search, page=page, per_page=per_page)
    data["users"] = data.pop("items")
    return data


def get_list_signal() -> dict:
    """Señal barata de cambios {count, version} para refresh on-change."""
    return _service.list_signal()


def get_user(user_id: str) -> User:
    try:
        return User.objects.get(pk=user_id)
    except User.DoesNotExist as exc:
        raise UserError(f"Usuario {user_id} no existe.") from exc


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
    """Crea un user nuevo. hashed_password viene de UserForm.hashed_password()."""
    if User.objects.filter(email__iexact=email).exists():
        raise UserError(f"Ya existe un usuario con email {email}.")

    if not Role.objects.filter(slug=role_slug, status="active").exists():
        raise UserError(f"Rol '{role_slug}' no existe o no está activo.")

    return User.objects.create(
        email=email,
        name=name or "",
        role=role_slug,
        status=status,
        password_hash=hashed_password,
    )


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
    """Soft delete: marca deleted_at + status=revoked. NO borra físicamente."""
    from django.utils import timezone
    user.deleted_at = timezone.now()
    user.status = "revoked"
    user.save()


@transaction.atomic
def toggle_user_status(user: User) -> str:
    """Alterna active <-> suspended (pending/revoked -> active). Devuelve el nuevo status."""
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


def get_stats() -> dict:
    """Stats agregadas para el header del listado."""
    return {
        "total": User.objects.count(),
        "active": User.objects.filter(status="active", deleted_at__isnull=True, is_erased=False).count(),
        "pending": User.objects.filter(status="pending").count(),
    }


# --- Alias para el descubrimiento por convención de core.views.MaintainerViews.
# entity_label="Usuario" -> _entity_attr() == "usuario", core busca
# get_usuario / create_usuario / update_usuario / delete_usuario /
# toggle_usuario_status. Apuntamos esos nombres a las funciones reales.
get_usuario = get_user
create_usuario = create_user
update_usuario = update_user
delete_usuario = delete_user
toggle_usuario_status = toggle_user_status
ServiceError = UserError
