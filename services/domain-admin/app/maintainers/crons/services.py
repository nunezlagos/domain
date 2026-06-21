"""Capa de negocio del mantenedor de Crons (schedules), migrada a core.

list + signal se delegan a core.service.MaintainerService (sin reimplementar la
búsqueda/paginación ni el aggregate de la señal). El listado excluye los
soft-deleted vía un queryset base. El resto —unicidad de slug, parseo de
inputs, create/update/delete/toggle del flag `enabled`— sigue acá.

Las views (core.views.MaintainerViews) descubren las funciones por convención
de nombre: entity_label="Cron" -> attr "cron" -> get_cron / create_cron /
update_cron / delete_cron / get_list_signal. El toggle alterna el flag booleano
`enabled` (no `status`), así que se sobreescribe el hook do_toggle en la view.
"""
from __future__ import annotations

from django.db import transaction

from core.service import MaintainerService

from .models import Cron


# Error de dominio (la view lo traduce a messages.error).
class CronError(Exception):
    """Error de operación sobre crons."""


# Service base reusado: list (search name/slug/expr/target_type + paginación) +
# signal. El queryset base excluye soft-deleted.
class CronsService(MaintainerService):
    model = Cron
    search_fields = ("name", "slug", "cron_expression", "target_type")
    ordering = ("-created_at",)

    def base_qs(self):
        return Cron.objects.filter(deleted_at__isnull=True)


_service = CronsService()


def list_crons(search: str = "", page: int = 1, per_page: int = 20) -> dict:
    """Lista crons (excluye soft-deleted) con búsqueda + paginación.

    Delega en MaintainerService.list y renombra `items` -> `crons` para no
    romper el contrato del template/tests existentes.
    """
    data = _service.list(qs=_service.base_qs(), search=search, page=page, per_page=per_page)
    data["crons"] = data.pop("items")
    return data


def get_list_signal() -> dict:
    """Señal barata de cambios {count, version} para refresh on-change."""
    return _service.list_signal()


def get_cron(cron_id: str) -> Cron:
    try:
        return Cron.objects.get(pk=cron_id)
    except Cron.DoesNotExist as exc:
        raise CronError(f"Cron {cron_id} no existe.") from exc


@transaction.atomic
def create_cron(
    *,
    name: str,
    slug: str,
    cron_expression: str,
    target_type: str,
    target_id: str,
    timezone: str = "UTC",
    description: str = "",
    inputs: dict | None = None,
    enabled: bool = True,
    created_by: str | None = None,
) -> Cron:
    """Crea un cron nuevo. slug debe ser único."""
    if Cron.objects.filter(slug=slug).exists():
        raise CronError(f"Ya existe un cron con slug '{slug}'.")

    return Cron.objects.create(
        created_by=created_by,
        name=name,
        slug=slug,
        description=description or "",
        cron_expression=cron_expression,
        timezone=timezone or "UTC",
        target_type=target_type,
        target_id=target_id,
        inputs=inputs if inputs is not None else {},
        enabled=enabled,
    )


@transaction.atomic
def update_cron(
    cron: Cron,
    *,
    name: str,
    slug: str,
    cron_expression: str,
    target_type: str,
    target_id: str,
    timezone: str = "UTC",
    description: str = "",
    inputs: dict | None = None,
    enabled: bool = True,
) -> Cron:
    """Actualiza un cron. El slug sigue siendo único."""
    if slug != cron.slug and Cron.objects.filter(slug=slug).exclude(pk=cron.pk).exists():
        raise CronError(f"Ya existe otro cron con slug '{slug}'.")

    cron.name = name
    cron.slug = slug
    cron.description = description or ""
    cron.cron_expression = cron_expression
    cron.timezone = timezone or "UTC"
    cron.target_type = target_type
    cron.target_id = target_id
    cron.inputs = inputs if inputs is not None else {}
    cron.enabled = enabled
    cron.save()
    return cron


@transaction.atomic
def delete_cron(cron: Cron) -> None:
    """Soft delete: marca deleted_at + deshabilita. NO borra físicamente.

    No hay status terminal en el flujo del cron; el registro queda
    deshabilitado (enabled=False) y fuera del listado (deleted_at != NULL).
    """
    from django.utils import timezone

    cron.deleted_at = timezone.now()
    cron.enabled = False
    cron.save()


@transaction.atomic
def toggle_cron_enabled(cron: Cron) -> bool:
    """Alterna enabled True <-> False. Retorna el nuevo valor de enabled."""
    cron.enabled = not cron.enabled
    cron.save()
    return cron.enabled


def get_stats() -> dict:
    """Stats agregadas para el header del listado."""
    base = Cron.objects.filter(deleted_at__isnull=True)
    return {
        "total": base.count(),
        "enabled": base.filter(enabled=True).count(),
        "disabled": base.filter(enabled=False).count(),
    }


# Alias para el descubrimiento por convención de core.views.MaintainerViews.
ServiceError = CronError
