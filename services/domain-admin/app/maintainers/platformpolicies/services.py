"""Capa de negocio del mantenedor de Politicas de plataforma.

list + signal se delegan a core.service.MaintainerService.
entity_label="Politica" -> core busca get_politica/create_politica/
update_politica/delete_politica/toggle_politica_status.
"""
from __future__ import annotations

from django.db import transaction
from django.utils import timezone

from core.service import MaintainerService

from .models import PlatformPolicy


class PlatformPolicyError(Exception):
    """Error de operacion sobre politicas de plataforma."""


class PlatformPolicyService(MaintainerService):
    model = PlatformPolicy
    search_fields = ("name", "slug", "body_md")
    ordering = ("-created_at",)

    def list(self, *args, **kwargs):
        kwargs.setdefault("qs", PlatformPolicy.objects.filter(deleted_at__isnull=True))
        return super().list(*args, **kwargs)


_service = PlatformPolicyService()


def list_policies(search: str = "", page: int = 1, per_page: int = 20) -> dict:
    data = _service.list(search=search, page=page, per_page=per_page)
    data["policies"] = data.pop("items")
    return data


def get_list_signal() -> dict:
    return _service.list_signal()


def get_policy(policy_id: str) -> PlatformPolicy:
    try:
        return PlatformPolicy.objects.get(pk=policy_id)
    except PlatformPolicy.DoesNotExist as exc:
        raise PlatformPolicyError(f"Politica {policy_id} no existe.") from exc


def _slug_taken(slug: str, exclude_pk=None) -> bool:
    qs = PlatformPolicy.objects.filter(slug=slug, deleted_at__isnull=True)
    if exclude_pk is not None:
        qs = qs.exclude(pk=exclude_pk)
    return qs.exists()


@transaction.atomic
def create_policy(
    *,
    slug: str,
    name: str,
    kind: str,
    body_md: str,
    is_active: bool = True,
) -> PlatformPolicy:
    if _slug_taken(slug):
        raise PlatformPolicyError(f"Ya existe una politica con slug '{slug}' en plataforma.")
    return PlatformPolicy.objects.create(
        slug=slug,
        name=name,
        kind=kind,
        body_md=body_md,
        is_active=is_active,
    )


@transaction.atomic
def update_policy(
    policy: PlatformPolicy,
    *,
    slug: str,
    name: str,
    kind: str,
    body_md: str,
    is_active: bool = True,
) -> PlatformPolicy:
    if _slug_taken(slug, exclude_pk=policy.pk):
        raise PlatformPolicyError(f"Ya existe otra politica con slug '{slug}' en plataforma.")
    policy.slug = slug
    policy.name = name
    policy.kind = kind
    policy.body_md = body_md
    policy.is_active = is_active
    policy.save()
    return policy


@transaction.atomic
def delete_policy(policy: PlatformPolicy) -> None:
    policy.deleted_at = timezone.now()
    policy.is_active = False
    policy.save()


@transaction.atomic
def toggle_policy_status(policy: PlatformPolicy) -> bool:
    if policy.is_active:
        policy.is_active = False
    else:
        policy.is_active = True
        policy.deleted_at = None
    policy.save()
    return policy.is_active


def get_stats() -> dict:
    base = PlatformPolicy.objects.filter(deleted_at__isnull=True)
    return {
        "total": base.count(),
        "active": base.filter(is_active=True).count(),
        "inactive": base.filter(is_active=False).count(),
    }


get_politica = get_policy
create_politica = create_policy
update_politica = update_policy
delete_politica = delete_policy
toggle_politica_status = toggle_policy_status
ServiceError = PlatformPolicyError
