"""Capa de negocio del mantenedor de API Keys (migrada a core).

list + signal se delegan a core.service.MaintainerService (sin reimplementar la
busqueda/paginacion ni el aggregate de la señal). El resto —generacion del
secreto, create/update/delete/toggle con sus validaciones de dominio— sigue aqui.

Las views (core.views.MaintainerViews, via ApiKeyViews) descubren las funciones
por convencion: get_api_key / create_api_key / update_api_key / delete_api_key /
toggle_api_key_status / get_list_signal. `entity_label="API Key"` ->
_entity_attr() == "api_key", por eso esos nombres calzan directo.
"""
from __future__ import annotations

import hashlib
import secrets

from django.db import transaction

from core.service import MaintainerService

from .models import ApiKey


# Error de dominio (la view lo traduce a messages.error).
class ApiKeyError(Exception):
    """Error de operacion sobre API keys."""


# Service base reusado: list (search name/prefix + paginacion) + signal.
class ApiKeyService(MaintainerService):
    model = ApiKey
    search_fields = ("name", "key_prefix")
    ordering = ("-created_at",)


_service = ApiKeyService()


def list_api_keys(search: str = "", page: int = 1, per_page: int = 20) -> dict:
    """Lista API keys con busqueda + paginacion.

    Delega en MaintainerService.list y renombra la clave `items` -> `api_keys`
    para no romper el contrato del template/tests existentes.
    """
    data = _service.list(search=search, page=page, per_page=per_page)
    data["api_keys"] = data.pop("items")
    return data


def get_list_signal() -> dict:
    """Señal barata de cambios {count, version} para refresh on-change."""
    return _service.list_signal()


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
    status: str = "active",
) -> tuple[ApiKey, str]:
    """Crea una API key nueva.

    Genera el secreto, persiste prefix + hash. Retorna (obj, secreto_claro).
    El secreto claro se muestra una sola vez en el front; no se vuelve a ver.
    """
    name = (name or "").strip()
    if not name:
        raise ApiKeyError("El nombre de la API Key es obligatorio.")
    if ApiKey.objects.filter(name=name).exists():
        raise ApiKeyError(f"Ya existe una API Key con nombre '{name}'.")

    full, prefix, key_hash = _generate_secret()
    api_key = ApiKey.objects.create(
        user=user,
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

    NO regenera el secreto (eso es rotacion, fuera de alcance aqui). Solo
    nombre, expiracion y status editables.
    """
    name = (name or "").strip()
    if not name:
        raise ApiKeyError("El nombre de la API Key es obligatorio.")
    clash = ApiKey.objects.filter(name=name).exclude(pk=api_key.pk)
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
    """Soft delete: revoca (marca revoked_at + status). NO borra fisicamente."""
    from django.utils import timezone

    api_key.revoked_at = timezone.now()
    api_key.status = "revoked"
    api_key.save()


@transaction.atomic
def toggle_api_key_status(api_key: ApiKey) -> str:
    """Alterna active <-> revoked. Retorna el nuevo status.

    Revocar setea revoked_at; reactivar lo limpia. NO se usa el toggle generico
    de core (active<->suspended) porque el dominio de keys es active<->revoked
    con manejo de revoked_at.
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


def get_stats() -> dict:
    """Stats agregadas para el header del listado."""
    total = ApiKey.objects.count()
    active = ApiKey.objects.filter(status="active", revoked_at__isnull=True).count()
    revoked = ApiKey.objects.filter(status="revoked").count()
    return {"total": total, "active": active, "revoked": revoked}


# Excepcion de dominio que core.views.MaintainerViews captura (error_class).
ServiceError = ApiKeyError
