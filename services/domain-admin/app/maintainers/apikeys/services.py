from __future__ import annotations

import base64
import secrets

import bcrypt
from django.conf import settings
from django.db import transaction

from core.service import MaintainerService
from maintainers.apikeys.repository import get_repository

from .models import ApiKey


_API_KEY_ENV = "live"
_PREFIX_LEN = 16
_SECRET_BYTES = 32
_BCRYPT_COST = 12


class ApiKeyError(Exception):
    pass


class ApiKeyService(MaintainerService):
    model = ApiKey
    search_fields = ("name", "key_prefix")
    ordering = ("-created_at",)


_service = ApiKeyService()


def _field_enc_key() -> str:
    key = getattr(settings, "FIELD_ENC_KEY", "") or ""
    if not key:
        raise ApiKeyError(
            "Cifrado de API keys no configurado: falta DOMAIN_FIELD_ENC_KEY en el env."
        )
    return key


def _generate_secret() -> tuple[str, str, bytes]:
    secret = secrets.token_bytes(_SECRET_BYTES)
    encoded = base64.urlsafe_b64encode(secret).rstrip(b"=").decode("ascii")
    full = f"domk_{_API_KEY_ENV}_{encoded}"
    prefix = full[:_PREFIX_LEN]
    key_hash = bcrypt.hashpw(full.encode("utf-8"), bcrypt.gensalt(rounds=_BCRYPT_COST))
    return full, prefix, key_hash


def list_api_keys(search: str = "", page: int = 1, per_page: int = 20,
                  user_id=None, status=None) -> dict:
    qs = ApiKey.objects.select_related("user").all()
    if user_id:
        qs = qs.filter(user_id=user_id)
    if status:
        qs = qs.filter(status=status)
    data = _service.list(qs=qs, search=search, page=page, per_page=per_page)
    data["api_keys"] = data.pop("items")
    return data


def export_api_keys_csv(search: str = "", user_id=None, status=None) -> str:
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
    return _service.list_signal()


def get_api_key(api_key_id: str) -> ApiKey:
    try:
        return ApiKey.objects.select_related("user").get(pk=api_key_id)
    except ApiKey.DoesNotExist as exc:
        raise ApiKeyError(f"API Key {api_key_id} no existe.") from exc


@transaction.atomic
def create_api_key(
    *,
    user,
    name: str,
    expires_at=None,
    status: str = "active",
) -> tuple[ApiKey, str]:
    name = (name or "").strip()
    if not name:
        raise ApiKeyError("El nombre de la API Key es obligatorio.")
    if ApiKey.objects.filter(name=name).exists():
        raise ApiKeyError(f"Ya existe una API Key con nombre '{name}'.")

    enc_key = _field_enc_key()
    full, prefix, key_hash = _generate_secret()

    api_key = ApiKey.objects.create(
        user=user,
        name=name,
        key_prefix=prefix,
        key_hash=key_hash,
        expires_at=expires_at,
        status=status,
    )

    repo = get_repository()
    repo.encrypt_and_store_ciphertext(str(api_key.pk), full, enc_key)
    return api_key, full


def get_api_key_plaintext(api_key_id: str) -> str | None:
    repo = get_repository()
    has_ciphertext, plaintext = repo.has_ciphertext(api_key_id)

    if has_ciphertext:
        try:
            enc_key = _field_enc_key()
            return repo.decrypt_ciphertext(api_key_id, enc_key)
        except Exception:
            return None

    return plaintext


@transaction.atomic
def update_api_key(
    api_key: ApiKey,
    *,
    name: str,
    expires_at=None,
    status: str | None = None,
) -> ApiKey:
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
    api_key.delete()


@transaction.atomic
def toggle_api_key_status(api_key: ApiKey) -> str:
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
    total = ApiKey.objects.count()
    active = ApiKey.objects.filter(status="active", revoked_at__isnull=True).count()
    revoked = ApiKey.objects.filter(status="revoked").count()
    return {"total": total, "active": active, "revoked": revoked}


ServiceError = ApiKeyError
