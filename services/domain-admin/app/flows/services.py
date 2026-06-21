"""Capa de negocio del mantenedor de Flows.

Patrón: las views solo hacen HTTP request/response; toda la lógica de
modelo vive acá. La tabla `flows` la administra domain-mcp (managed=False);
Django solo lee/escribe vía ORM.

Particularidades de `flows` respecto del patrón base:
- El estado habilitado/deshabilitado es el boolean `is_active`. El toggle
  alterna ese boolean.
- Soft-delete vía deleted_at (+ is_active=false al eliminar).
- flow_versions es un sub-recurso READ-ONLY (sin CRUD); se expone solo
  un getter para mostrarlo en el detalle del flow.
"""
from __future__ import annotations

from django.db import transaction

from .models import Flow, FlowVersion


# Error de dominio (la view lo traduce a messages.error).
class FlowError(Exception):
    """Error de operación sobre flows."""


def list_flows(search: str = "", page: int = 1, per_page: int = 20) -> dict:
    """Lista flows con búsqueda opcional + paginación.

    Excluye los soft-deleted (deleted_at != NULL). Búsqueda sobre
    name / slug / description.

    Retorna dict con: flows, total, page, per_page, total_pages,
    has_next, has_prev.
    """
    qs = Flow.objects.filter(deleted_at__isnull=True)
    if search:
        qs = (
            qs.filter(name__icontains=search)
            | qs.filter(slug__icontains=search)
            | qs.filter(description__icontains=search)
        )
    qs = qs.distinct().order_by("-created_at")

    total = qs.count()
    total_pages = max(1, (total + per_page - 1) // per_page)
    start = (page - 1) * per_page
    end = start + per_page
    flows = list(qs[start:end])

    return {
        "flows": flows,
        "total": total,
        "page": page,
        "per_page": per_page,
        "total_pages": total_pages,
        "has_next": end < total,
        "has_prev": page > 1,
    }


def get_flow(flow_id: str) -> Flow:
    try:
        return Flow.objects.get(pk=flow_id)
    except Flow.DoesNotExist as exc:
        raise FlowError(f"Flow {flow_id} no existe.") from exc


def get_flow_versions(flow: Flow) -> list[FlowVersion]:
    """Sub-recurso READ-ONLY: versiones del flow ordenadas (más nueva primero)."""
    return list(FlowVersion.objects.filter(flow=flow).order_by("-version"))


@transaction.atomic
def create_flow(
    *,
    name: str,
    slug: str,
    description: str = "",
    spec: dict | None = None,
    is_active: bool = True,
    deterministic_replay: bool = False,
    seed_managed: bool = False,
    seed_version: int | None = None,
) -> Flow:
    """Crea un flow nuevo. slug debe ser único."""
    if Flow.objects.filter(slug=slug).exists():
        raise FlowError(f"Ya existe un flow con slug '{slug}'.")

    flow = Flow.objects.create(
        name=name,
        slug=slug,
        description=description or "",
        spec=spec if spec is not None else {},
        is_active=is_active,
        deterministic_replay=deterministic_replay,
        seed_managed=seed_managed,
        seed_version=seed_version,
    )
    return flow


@transaction.atomic
def update_flow(
    flow: Flow,
    *,
    name: str,
    slug: str,
    description: str = "",
    spec: dict | None = None,
    is_active: bool = True,
    deterministic_replay: bool = False,
    seed_managed: bool = False,
    seed_version: int | None = None,
) -> Flow:
    """Actualiza un flow. El slug sigue siendo único."""
    if slug != flow.slug and Flow.objects.filter(
        slug=slug
    ).exclude(pk=flow.pk).exists():
        raise FlowError(f"Ya existe otro flow con slug '{slug}'.")

    flow.name = name
    flow.slug = slug
    flow.description = description or ""
    if spec is not None:
        flow.spec = spec
    flow.is_active = is_active
    flow.deterministic_replay = deterministic_replay
    flow.seed_managed = seed_managed
    flow.seed_version = seed_version
    flow.save()
    return flow


@transaction.atomic
def delete_flow(flow: Flow) -> None:
    """Soft delete: marca deleted_at + is_active=false. NO borra físicamente."""
    from django.utils import timezone

    flow.deleted_at = timezone.now()
    flow.is_active = False
    flow.save()


@transaction.atomic
def toggle_flow_status(flow: Flow) -> bool:
    """Alterna is_active (habilitado <-> deshabilitado). Retorna el nuevo valor."""
    flow.is_active = not flow.is_active
    flow.save()
    return flow.is_active


def get_list_signal() -> dict:
    """Señal barata de cambios para refresh on-change.

    Devuelve count + max(updated_at): cualquier alta, edición, baja (soft)
    o toggle muta uno de los dos (updated_at lo bumpea el trigger
    set_updated_at en la BD; created_at de altas nuevas sube el max). El
    front compara contra su última señal y solo re-renderiza la tabla
    cuando algo cambió en la BD — incluyendo inserts de otros servicios
    (domain-mcp) que escriben directo en `flows`.

    Query barata: SELECT count(*), max(updated_at) FROM flows.
    """
    from django.db.models import Count, Max

    agg = Flow.objects.aggregate(total=Count("id"), latest=Max("updated_at"))
    latest = agg["latest"]
    return {
        "count": agg["total"] or 0,
        "version": latest.isoformat() if latest else "",
    }


def get_stats() -> dict:
    """Stats agregadas para el header del listado."""
    base = Flow.objects.filter(deleted_at__isnull=True)
    return {
        "total": base.count(),
        "active": base.filter(is_active=True).count(),
        "inactive": base.filter(is_active=False).count(),
    }
