"""Capa de negocio del mantenedor de Reglas por proyecto.

list + signal se delegan a core.service.MaintainerService. Lo propio —unicidad
(project_id, slug) entre activas, toggle is_active que reactiva soft-deleted—
queda aqui. entity_label="Regla" -> core busca get_regla/create_regla/
update_regla/delete_regla/toggle_regla_status: se exponen como alias.
"""
from __future__ import annotations

from django.db import transaction
from django.utils import timezone

from core.service import MaintainerService

from .models import ProjectPolicy


class ProjectPolicyError(Exception):
    """Error de operacion sobre reglas de proyecto."""


class ProjectPolicyService(MaintainerService):
    model = ProjectPolicy
    search_fields = ("name", "slug", "body_md")
    ordering = ("-created_at",)

    def list(self, *args, **kwargs):
        kwargs.setdefault("qs", ProjectPolicy.objects.filter(deleted_at__isnull=True))
        return super().list(*args, **kwargs)


_service = ProjectPolicyService()


def list_policies(search: str = "", page: int = 1, per_page: int = 20) -> dict:
    data = _service.list(search=search, page=page, per_page=per_page)
    data["policies"] = data.pop("items")
    return data


def get_list_signal() -> dict:
    return _service.list_signal()


def get_policy(policy_id: str) -> ProjectPolicy:
    try:
        return ProjectPolicy.objects.get(pk=policy_id)
    except ProjectPolicy.DoesNotExist as exc:
        raise ProjectPolicyError(f"Regla {policy_id} no existe.") from exc


def _slug_taken(project_id, slug: str, exclude_pk=None) -> bool:
    qs = ProjectPolicy.objects.filter(
        project_id=project_id, slug=slug, deleted_at__isnull=True
    )
    if exclude_pk is not None:
        qs = qs.exclude(pk=exclude_pk)
    return qs.exists()


@transaction.atomic
def create_policy(
    *,
    project_id,
    slug: str,
    name: str,
    kind: str,
    body_md: str,
    override_platform: bool = False,
    is_active: bool = True,
) -> ProjectPolicy:
    if not project_id:
        raise ProjectPolicyError("La regla requiere un proyecto.")
    if _slug_taken(project_id, slug):
        raise ProjectPolicyError(f"Ya existe una regla con slug '{slug}' en este proyecto.")
    return ProjectPolicy.objects.create(
        project_id=project_id,
        slug=slug,
        name=name,
        kind=kind,
        body_md=body_md,
        override_platform=override_platform,
        is_active=is_active,
        source="dashboard",
    )


@transaction.atomic
def update_policy(
    policy: ProjectPolicy,
    *,
    slug: str,
    name: str,
    kind: str,
    body_md: str,
    override_platform: bool = False,
    is_active: bool = True,
) -> ProjectPolicy:
    if _slug_taken(policy.project_id, slug, exclude_pk=policy.pk):
        raise ProjectPolicyError(f"Ya existe otra regla con slug '{slug}' en este proyecto.")
    policy.slug = slug
    policy.name = name
    policy.kind = kind
    policy.body_md = body_md
    policy.override_platform = override_platform
    policy.is_active = is_active
    policy.save()
    return policy


@transaction.atomic
def delete_policy(policy: ProjectPolicy) -> None:
    policy.deleted_at = timezone.now()
    policy.is_active = False
    policy.save()


@transaction.atomic
def toggle_policy_status(policy: ProjectPolicy) -> bool:
    if policy.is_active:
        policy.is_active = False
    else:
        policy.is_active = True
        policy.deleted_at = None
    policy.save()
    return policy.is_active


def get_stats() -> dict:
    base = ProjectPolicy.objects.filter(deleted_at__isnull=True)
    return {
        "total": base.count(),
        "active": base.filter(is_active=True).count(),
        "inactive": base.filter(is_active=False).count(),
    }


# --- Alias para el descubrimiento por convencion (entity_label="Regla").
get_regla = get_policy
create_regla = create_policy
update_regla = update_policy
delete_regla = delete_policy
toggle_regla_status = toggle_policy_status
ServiceError = ProjectPolicyError
