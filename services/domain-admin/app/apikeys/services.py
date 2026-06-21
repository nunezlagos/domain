"""HU-API.1: services (capa de negocio, separada de views).

Patrón: las views solo hacen HTTP request/response; toda la lógica de
modelo vive acá. Esto facilita testing unitario sin tocar HTTP.

Funciones canónicas:
    list_api_keys(search, page, per_page) -> dict
    get_api_key(id) -> ApiKey
    create_api_key(**kw) -> ApiKey
    update_api_key(obj, **kw) -> ApiKey
    delete_api_key(obj) -> None            (soft delete: revoked_at + status)
    toggle_api_key_status(obj) -> str       (active <-> revoked)
    get_list_signal() -> dict               (count + max(updated_at))
    get_stats() -> dict
"""
from __future__ import annotations

import hashlib
import secrets

from django.db import transaction

from .models import ApiKey


# Errores de dominio (la view los traduce a messages.error).
class ApiKeyError(Exception):
    """Error de operación sobre API keys."""


def list_api_keys(search: str = "", page: int = 1, per_page: int = 20) -> dict:
    """Lista API keys con búsqueda opcional + paginación.

    Retorna dict con: api_keys, total, page, per_page, total_pages,
    has_next, has_prev.
    """
    qs = ApiKey.objects.all()
    if search:
        qs = qs.filter(name__icontains=search) | qs.filter(key_prefix__icontains=search)
    qs = qs.distinct().order_by("-created_at")

    total = qs.count()
    total_pages = max(1, (total + per_page - 1) // per_page)
    start = (page - 1) * per_page
    end = start + per_page
    api_keys = list(qs.select_related("user")[start:end])

    return {
        "api_keys": api_keys,
        "total": total,
        "page": page,
        "per_page": per_page,
        "total_pages": total_pages,
        "has_next": end < total,
        "has_prev": page > 1,
    }


def get_api_key(api_key_id: str) -> ApiKey:
    try:
        return ApiKey.objects.select_related("user").get(pk=api_key_id)
    except ApiKey.DoesNotExist as exc:
        raise ApiKeyError(f"API Key {api_key_id} no existe.") from exc


def _generate_secret() -> tuple[str, str, bytes]:
    """Genera un secreto nuevo. Retorna (secreto_claro, prefix, hash).

    El secreto en claro SOLO se devuelve una vez (al crear) para mostrarlo al
    usuario; nunca se persiste. Se guarda prefix (visible) + hash (sha256).
    """
    raw = secrets.token_urlsafe(32)
    full = f"sk_{raw}"
    prefix = full[:20]
    key_hash = hashlib.sha256(full.encode("utf-8")).digest()
    return full, prefix, key_hash


@transaction.atomic
def create_api_key(
    *,
    user,
    name: str,
    expires_at=None,
    organization_id=None,
    status: str = "active",
) -> tuple[ApiKey, str]:
    """Crea una API key nueva.

    Genera el secreto, persiste prefix + hash. Retorna (obj, secreto_claro).
    El secreto claro se muestra una sola vez en el front; no se vuelve a ver.
    """
    name = (name or "").strip()
    if not name:
        raise ApiKeyError("El nombre de la API Key es obligatorio.")
    if ApiKey.objects.filter(name=name, organization_id=organization_id).exists():
        raise ApiKeyError(f"Ya existe una API Key con nombre '{name}'.")

    full, prefix, key_hash = _generate_secret()
    api_key = ApiKey.objects.create(
        user=user,
        organization_id=organization_id,
        name=name,
        key_prefix=prefix,
        key_hash=key_hash,
        expires_at=expires_at,
        status=status,
    )
    return api_key, full


@transaction.atomic
def update_api_key(
    api_key: ApiKey,
    *,
    name: str,
    expires_at=None,
    status: str | None = None,
) -> ApiKey:
    """Actualiza metadata de la API key.

    NO regenera el secreto (eso es rotación, fuera de alcance acá). Solo
    nombre, expiración y status editables.
    """
    name = (name or "").strip()
    if not name:
        raise ApiKeyError("El nombre de la API Key es obligatorio.")
    clash = (
        ApiKey.objects.filter(name=name, organization_id=api_key.organization_id)
        .exclude(pk=api_key.pk)
    )
    if clash.exists():
        raise ApiKeyError(f"Ya existe otra API Key con nombre '{name}'.")

    api_key.name = name
    api_key.expires_at = expires_at
    if status is not None:
        api_key.status = status
    api_key.save()
    return api_key


@transaction.atomic
def delete_api_key(api_key: ApiKey) -> None:
    """Soft delete: revoca (marca revoked_at + status). NO borra físicamente."""
    from django.utils import timezone

    api_key.revoked_at = timezone.now()
    api_key.status = "revoked"
    api_key.save()


@transaction.atomic
def toggle_api_key_status(api_key: ApiKey) -> str:
    """Alterna active <-> revoked. Retorna el nuevo status.

    Revocar setea revoked_at; reactivar lo limpia.
    """
    from django.utils import timezone

    if api_key.status == "active" and api_key.revoked_at is None:
        api_key.status = "revoked"
        api_key.revoked_at = timezone.now()
    else:
        api_key.status = "active"
        api_key.revoked_at = None
    api_key.save()
    return api_key.status


def get_list_signal() -> dict:
    """Señal barata de cambios para refresh on-change.

    Devuelve count + max(updated_at): cualquier alta, edición, baja (soft)
    o toggle muta uno de los dos (updated_at es auto_now; created_at de altas
    nuevas sube el max). El front compara contra su última señal y solo
    re-renderiza la tabla cuando algo cambió en la BD — incluyendo inserts de
    otros servicios (domain-mcp) que escriben directo en `auth_api_keys`.

    Query barata: SELECT count(*), max(updated_at) FROM auth_api_keys.
    """
    from django.db.models import Count, Max

    agg = ApiKey.objects.aggregate(total=Count("id"), latest=Max("updated_at"))
    latest = agg["latest"]
    return {
        "count": agg["total"] or 0,
        "version": latest.isoformat() if latest else "",
    }


def get_stats() -> dict:
    """Stats agregadas para el header del listado."""
    total = ApiKey.objects.count()
    active = ApiKey.objects.filter(status="active", revoked_at__isnull=True).count()
    revoked = ApiKey.objects.filter(status="revoked").count()
    return {"total": total, "active": active, "revoked": revoked}
