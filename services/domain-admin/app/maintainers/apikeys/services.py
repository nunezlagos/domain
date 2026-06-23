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

import base64
import secrets

import bcrypt
from django.conf import settings
from django.db import connection, transaction

from core.service import MaintainerService

from .models import ApiKey


def _field_enc_key() -> str:
    """Passphrase de cifrado at-rest (DOMAIN_FIELD_ENC_KEY via settings).

    Compartida con domain-mcp; DEBE ser identica en ambos .env. Si esta vacia no
    se puede cifrar/descifrar: fallamos antes que escribir/leer en claro.
    """
    key = getattr(settings, "FIELD_ENC_KEY", "") or ""
    if not key:
        raise ApiKeyError(
            "Cifrado de API keys no configurado: falta DOMAIN_FIELD_ENC_KEY en el env."
        )
    return key


# Error de dominio (la view lo traduce a messages.error).
class ApiKeyError(Exception):
    """Error de operacion sobre API keys."""


# Service base reusado: list (search name/prefix + paginacion) + signal.
class ApiKeyService(MaintainerService):
    model = ApiKey
    search_fields = ("name", "key_prefix")
    ordering = ("-created_at",)


_service = ApiKeyService()


def list_api_keys(search: str = "", page: int = 1, per_page: int = 20,
                  user_id=None, status=None) -> dict:
    """Lista API keys con busqueda + filtros (usuario/estado) + paginacion.

    Delega en MaintainerService.list (qs pre-filtrado) y renombra `items` ->
    `api_keys` para no romper el contrato del template/tests existentes.
    """
    qs = ApiKey.objects.select_related("user").all()
    if user_id:
        qs = qs.filter(user_id=user_id)
    if status:
        qs = qs.filter(status=status)
    data = _service.list(qs=qs, search=search, page=page, per_page=per_page)
    data["api_keys"] = data.pop("items")
    return data


def export_api_keys_csv(search: str = "", user_id=None, status=None) -> str:
    """CSV consolidado (compatible con Excel) de las API keys que matchean los
    filtros activos (usuario/estado/busqueda). Sin paginar."""
    import csv
    import io
    from django.db.models import Q

    qs = ApiKey.objects.select_related("user").all()
    if user_id:
        qs = qs.filter(user_id=user_id)
    if status:
        qs = qs.filter(status=status)
    if search:
        qs = qs.filter(Q(name__icontains=search) | Q(key_prefix__icontains=search))
    qs = qs.order_by("name")

    buf = io.StringIO()
    w = csv.writer(buf)
    w.writerow(["Prefijo", "Nombre", "Usuario", "Estado", "Ultimo uso", "Expira", "Creado"])
    for k in qs:
        w.writerow([
            k.key_prefix, k.name,
            k.user.email if k.user_id else "",
            k.get_status_display(),
            k.last_used_at.strftime("%Y-%m-%d %H:%M") if k.last_used_at else "",
            k.expires_at.strftime("%Y-%m-%d %H:%M") if k.expires_at else "",
            k.created_at.strftime("%Y-%m-%d %H:%M") if k.created_at else "",
        ])
    return buf.getvalue()


def get_list_signal() -> dict:
    """Señal barata de cambios {count, version} para refresh on-change."""
    return _service.list_signal()


def get_api_key(api_key_id: str) -> ApiKey:
    try:
        return ApiKey.objects.select_related("user").get(pk=api_key_id)
    except ApiKey.DoesNotExist as exc:
        raise ApiKeyError(f"API Key {api_key_id} no existe.") from exc


# Constantes espejadas del backend Go (internal/auth/apikey/apikey.go).
# DEBEN coincidir o el MCP rechaza la key: formato domk_<env>_<secret>,
# prefix = primeros 16 chars, hash = bcrypt(cost=12).
_API_KEY_ENV = "live"
_PREFIX_LEN = 16
_SECRET_BYTES = 32
_BCRYPT_COST = 12


def _generate_secret() -> tuple[str, str, bytes]:
    """Genera un secreto nuevo. Retorna (secreto_claro, prefix, hash).

    Formato IDENTICO al backend Go: `domk_<env>_<base64url(32 bytes, sin
    padding)>`. El hash es bcrypt (lo que valida Resolve en el MCP). El plaintext
    se devuelve para mostrarlo y persistirlo (key_plaintext) — el owner pidio
    poder verlo de nuevo.
    """
    secret = secrets.token_bytes(_SECRET_BYTES)
    encoded = base64.urlsafe_b64encode(secret).rstrip(b"=").decode("ascii")
    full = f"domk_{_API_KEY_ENV}_{encoded}"
    prefix = full[:_PREFIX_LEN]
    key_hash = bcrypt.hashpw(full.encode("utf-8"), bcrypt.gensalt(rounds=_BCRYPT_COST))
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

    # La passphrase se valida ANTES de generar/crear nada: si no esta, abortamos
    # sin tocar la DB (no queremos una key a medio crear).
    enc_key = _field_enc_key()

    full, prefix, key_hash = _generate_secret()
    # key_plaintext queda NULL (mig 000168): ya no se escribe la key en claro.
    api_key = ApiKey.objects.create(
        user=user,
        name=name,
        key_prefix=prefix,
        key_hash=key_hash,
        expires_at=expires_at,
        status=status,
    )
    # Cifrado at-rest: key_ciphertext = pgp_sym_encrypt(full, passphrase). Raw SQL
    # porque la columna es BYTEA y el cifrado lo hace Postgres (pgcrypto), no el
    # ORM. Va dentro del mismo @transaction.atomic que el create de arriba.
    with connection.cursor() as cursor:
        cursor.execute(
            "UPDATE auth_api_keys SET key_ciphertext = pgp_sym_encrypt(%s, %s) "
            "WHERE id = %s",
            [full, enc_key, str(api_key.pk)],
        )
    return api_key, full


def get_api_key_plaintext(api_key_id: str) -> str | None:
    """Devuelve la key en claro descifrada, o None si no hay nada que mostrar.

    Prioridad:
      1) key_ciphertext (keys nuevas, mig 000168): pgp_sym_decrypt(..., passphrase).
      2) key_plaintext (fallback keys viejas creadas antes de la mig).
      3) None (solo se vio el prefijo; nunca se guardo el secreto).

    El descifrado lo hace Postgres (pgcrypto) con la passphrase del env; la key
    nunca se persiste descifrada.
    """
    with connection.cursor() as cursor:
        cursor.execute(
            "SELECT key_ciphertext IS NOT NULL, key_plaintext FROM auth_api_keys "
            "WHERE id = %s",
            [str(api_key_id)],
        )
        row = cursor.fetchone()
    if row is None:
        return None
    has_ciphertext, plaintext = row
    if has_ciphertext:
        # Descifrado AISLADO en su propio savepoint: si la passphrase no coincide
        # (ej. tras rotar DOMAIN_FIELD_ENC_KEY) o falta, pgp_sym_decrypt lanza error
        # de DB. Lo capturamos y devolvemos None: el detalle del usuario descifra
        # varias keys en loop, y un error NO debe tumbar (500) toda la pagina.
        try:
            with transaction.atomic(), connection.cursor() as cursor:
                cursor.execute(
                    "SELECT pgp_sym_decrypt(key_ciphertext, %s)::text "
                    "FROM auth_api_keys WHERE id = %s",
                    [_field_enc_key(), str(api_key_id)],
                )
                return cursor.fetchone()[0]
        except Exception:
            return None
    # Fallback: key vieja en claro (puede ser None tambien).
    return plaintext


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
    """Hard delete: ELIMINA la fila (no es revocar). Revocar sin borrar queda en
    el toggle (active<->revoked). El usuario pidio que 'eliminar' elimine de verdad
    (desaparece de la lista), no que quede como revocada."""
    api_key.delete()


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
