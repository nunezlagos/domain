"""Capa de negocio del mantenedor de usuarios (migrada a core).

list + signal se delegan a core.service.MaintainerService (sin reimplementar la
busqueda/paginacion ni el aggregate de la señal). El resto —roles, password,
create/update/delete/toggle con sus validaciones de dominio— sigue aqui.

Las views (core.views.MaintainerViews) descubren las funciones por convencion
de nombre: get_user / create_user / update_user / delete_user /
toggle_user_status / get_list_signal. `entity_label="Usuario"` -> attr "usuario",
por eso ademas exponemos alias get_usuario/... para el descubrimiento del core.
"""
from __future__ import annotations

from django.db import transaction

from core.service import MaintainerService

from .models import Role, User, UserRole



class UserError(Exception):
    """Error de operacion sobre usuarios."""



class UserService(MaintainerService):
    model = User
    search_fields = ("email", "name")
    ordering = ("-created_at",)


_service = UserService()


def _filtered_user_qs(roles=None, statuses=None):
    """Queryset de User filtrado por rol y/o estado (multi-select). Listas vacias
    = sin filtro. Se pasa como qs base a MaintainerService.list (que suma search)."""
    qs = User.objects.all()
    if roles:
        qs = qs.filter(role__in=roles)
    if statuses:
        qs = qs.filter(status__in=statuses)
    return qs


def list_users(search: str = "", page: int = 1, per_page: int = 20,
               roles=None, statuses=None) -> dict:
    """Lista usuarios con busqueda + filtros (rol/estado) + paginacion.

    Delega en MaintainerService.list (pasando el qs ya filtrado) y renombra la
    clave `items` -> `users` para no romper el contrato del template/tests.
    """
    data = _service.list(qs=_filtered_user_qs(roles, statuses),
                         search=search, page=page, per_page=per_page)
    data["users"] = data.pop("items")
    return data


def export_users_csv(search: str = "", roles=None, statuses=None) -> str:
    """CSV consolidado (compatible con Excel) de los usuarios que matchean los
    filtros activos (rol/estado/busqueda). Sin paginar."""
    import csv
    import io
    from django.db.models import Q

    qs = _filtered_user_qs(roles, statuses)
    if search:
        qs = qs.filter(Q(email__icontains=search) | Q(name__icontains=search))
    qs = qs.distinct().order_by("email")

    buf = io.StringIO()
    w = csv.writer(buf)
    w.writerow(["Email", "Nombre", "Rol", "Estado", "RUT", "Ultimo acceso", "Creado"])
    for u in qs:
        w.writerow([
            u.email, u.name, u.role, u.get_status_display(), u.rut,
            u.last_login_at.strftime("%Y-%m-%d %H:%M") if u.last_login_at else "",
            u.created_at.strftime("%Y-%m-%d %H:%M") if u.created_at else "",
        ])
    return buf.getvalue()


def list_role_options() -> list[str]:
    """Roles distintos en uso (para el multi-select del filtro)."""
    return sorted(r for r in User.objects.values_list("role", flat=True).distinct() if r)


def get_list_signal() -> dict:
    """Señal barata de cambios {count, version} para refresh on-change."""
    return _service.list_signal()


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
    """Crea un user nuevo. hashed_password viene de UserForm.hashed_password()."""
    if User.objects.filter(email__iexact=email).exists():
        raise UserError(f"Ya existe un usuario con email {email}.")

    if not Role.objects.filter(slug=role_slug, status="active").exists():
        raise UserError(f"Rol '{role_slug}' no existe o no esta activo.")

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
        raise UserError(f"Rol '{role_slug}' no existe o no esta activo.")

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
    """Soft delete: marca deleted_at + status=revoked. NO borra fisicamente."""
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
    """Revoca un rol del user. Retorna True si se elimino."""
    deleted_count, _ = UserRole.objects.filter(user=user, role_id=role_id).delete()
    return deleted_count > 0


def get_stats() -> dict:
    """Stats agregadas para el header del listado."""
    return {
        "total": User.objects.count(),
        "active": User.objects.filter(status="active", deleted_at__isnull=True, is_erased=False).count(),
        "pending": User.objects.filter(status="pending").count(),
    }






get_usuario = get_user
create_usuario = create_user
update_usuario = update_user
delete_usuario = delete_user
toggle_usuario_status = toggle_user_status
ServiceError = UserError
