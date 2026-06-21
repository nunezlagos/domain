"""Capa de negocio del mantenedor de Crons (schedules).

Patrón: las views solo hacen HTTP request/response; toda la lógica de
modelo vive acá. Esto facilita testing unitario sin tocar HTTP.

La tabla `crons` la administra domain-mcp (managed=False); Django solo
lee/escribe vía ORM. Soft-delete (deleted_at) + toggle del flag booleano
`enabled` (habilitar/deshabilitar el schedule).
"""
from __future__ import annotations

from django.db import transaction

from .models import Cron


# Error de dominio (la view lo traduce a messages.error).
class CronError(Exception):
    """Error de operación sobre crons."""


def list_crons(search: str = "", page: int = 1, per_page: int = 20) -> dict:
    """Lista crons con búsqueda opcional + paginación.

    Excluye los soft-deleted (deleted_at != NULL). Búsqueda sobre
    name / slug / cron_expression / target_type.

    Retorna dict con: crons, total, page, per_page, total_pages,
    has_next, has_prev.
    """
    qs = Cron.objects.filter(deleted_at__isnull=True)
    if search:
        qs = (
            qs.filter(name__icontains=search)
            | qs.filter(slug__icontains=search)
            | qs.filter(cron_expression__icontains=search)
            | qs.filter(target_type__icontains=search)
        )
    qs = qs.distinct().order_by("-created_at")

    total = qs.count()
    total_pages = max(1, (total + per_page - 1) // per_page)
    start = (page - 1) * per_page
    end = start + per_page
    crons = list(qs[start:end])

    return {
        "crons": crons,
        "total": total,
        "page": page,
        "per_page": per_page,
        "total_pages": total_pages,
        "has_next": end < total,
        "has_prev": page > 1,
    }


def get_cron(cron_id: str) -> Cron:
    try:
        return Cron.objects.get(pk=cron_id)
    except Cron.DoesNotExist as exc:
        raise CronError(f"Cron {cron_id} no existe.") from exc


@transaction.atomic
def create_cron(
    *,
    organization_id: str,
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
    """Crea un cron nuevo. slug debe ser único dentro de la organización."""
    if Cron.objects.filter(organization_id=organization_id, slug=slug).exists():
        raise CronError(
            f"Ya existe un cron con slug '{slug}' en esta organización."
        )

    cron = Cron.objects.create(
        organization_id=organization_id,
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
    return cron


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
    """Actualiza un cron. El slug sigue siendo único per-organización."""
    if slug != cron.slug and Cron.objects.filter(
        organization_id=cron.organization_id, slug=slug
    ).exclude(pk=cron.pk).exists():
        raise CronError(
            f"Ya existe otro cron con slug '{slug}' en esta organización."
        )

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

    No hay status terminal en el schema; el cron queda deshabilitado
    (enabled=False) y fuera del listado (deleted_at != NULL).
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


def get_list_signal() -> dict:
    """Señal barata de cambios para refresh on-change.

    NO es polling ciego de la tabla. Devuelve count + max(updated_at):
    cualquier alta, edición, baja (soft) o toggle muta uno de los dos
    (updated_at lo bumpea el trigger set_updated_at en la BD; created_at
    de altas nuevas sube el max). El front compara contra su última señal
    y solo re-renderiza la tabla cuando algo cambió en la BD — incluyendo
    inserts de otros servicios (domain-mcp) que escriben directo en `crons`.

    Query barata: SELECT count(*), max(updated_at) FROM crons.
    """
    from django.db.models import Count, Max

    agg = Cron.objects.aggregate(total=Count("id"), latest=Max("updated_at"))
    latest = agg["latest"]
    return {
        "count": agg["total"] or 0,
        "version": latest.isoformat() if latest else "",
    }


def get_stats() -> dict:
    """Stats agregadas para el header del listado."""
    base = Cron.objects.filter(deleted_at__isnull=True)
    return {
        "total": base.count(),
        "enabled": base.filter(enabled=True).count(),
        "disabled": base.filter(enabled=False).count(),
    }
