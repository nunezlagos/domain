"""Capa de negocio del mantenedor de Flows (migrada a core).

list + signal se delegan a core.service.MaintainerService (sin reimplementar la
busqueda/paginacion ni el aggregate de la señal). El resto —spec JSONB, unicidad
de slug, toggle sobre el boolean is_active, soft-delete y las versiones
read-only— sigue aqui.

Particularidades de `flows` respecto del patron base:
- list excluye los soft-deleted (deleted_at != NULL) via un queryset base.
- El estado habilitado/deshabilitado es el boolean `is_active` (NO el campo
  status); el toggle alterna ese boolean.
- flow_versions es un sub-recurso READ-ONLY (sin CRUD); se expone solo un getter
  para mostrarlo en el detalle del flow (analogo a get_user_roles en users).

Las views (core.views.MaintainerViews) descubren las funciones por convencion
de nombre: get_flow / create_flow / update_flow / delete_flow /
toggle_flow_status / get_list_signal. `entity_label="Flow"` -> attr "flow", que
ya coincide con esos nombres.
"""
from __future__ import annotations

from django.db import transaction

from core.service import MaintainerService

from .models import Flow, FlowVersion


# Error de dominio (la view lo traduce a messages.error).
class FlowError(Exception):
    """Error de operacion sobre flows."""


# Service base reusado: list (search name/slug/description + paginacion) + signal.
class FlowService(MaintainerService):
    model = Flow
    search_fields = ("name", "slug", "description")
    ordering = ("-created_at",)


_service = FlowService()


def _filtered_flow_qs(is_active=None):
    """Queryset de Flow NO eliminado, filtrado por el boolean is_active. `None`
    = sin filtro de estado. Se pasa como qs base a MaintainerService.list (que
    suma search). Preserva la exclusion de soft-deleted (deleted_at != NULL)."""
    qs = Flow.objects.filter(deleted_at__isnull=True)
    if is_active is not None:
        qs = qs.filter(is_active=is_active)
    return qs


def list_flows(search: str = "", page: int = 1, per_page: int = 20,
               is_active=None) -> dict:
    """Lista flows NO eliminados con busqueda + filtro is_active + paginacion.

    Delega en MaintainerService.list (pasandole el queryset ya filtrado, que
    excluye los soft-deleted) y renombra la clave `items` -> `flows` para no
    romper el contrato del template/tests existentes.
    """
    data = _service.list(qs=_filtered_flow_qs(is_active),
                         search=search, page=page, per_page=per_page)
    data["flows"] = data.pop("items")
    return data


def export_flows_csv(search: str = "", is_active=None) -> str:
    """CSV consolidado (compatible con Excel) de los flows que matchean los
    filtros activos (estado/busqueda). Sin paginar."""
    import csv
    import io
    from django.db.models import Q

    qs = _filtered_flow_qs(is_active)
    if search:
        qs = qs.filter(
            Q(name__icontains=search)
            | Q(slug__icontains=search)
            | Q(description__icontains=search)
        )
    qs = qs.order_by("name")

    buf = io.StringIO()
    w = csv.writer(buf)
    w.writerow(["Nombre", "Slug", "Descripcion", "Activo", "Creado"])
    for f in qs:
        w.writerow([
            f.name, f.slug, f.description,
            "Si" if f.is_active else "No",
            f.created_at.strftime("%Y-%m-%d %H:%M") if f.created_at else "",
        ])
    return buf.getvalue()


def get_list_signal() -> dict:
    """Señal barata de cambios {count, version} para refresh on-change."""
    return _service.list_signal()


def get_flow(flow_id: str) -> Flow:
    try:
        return Flow.objects.get(pk=flow_id)
    except Flow.DoesNotExist as exc:
        raise FlowError(f"Flow {flow_id} no existe.") from exc


def get_flow_versions(flow: Flow) -> list[FlowVersion]:
    """Sub-recurso READ-ONLY: versiones del flow ordenadas (mas nueva primero)."""
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
    """Crea un flow nuevo. slug debe ser unico."""
    if Flow.objects.filter(slug=slug).exists():
        raise FlowError(f"Ya existe un flow con slug '{slug}'.")

    return Flow.objects.create(
        name=name,
        slug=slug,
        description=description or "",
        spec=spec if spec is not None else {},
        is_active=is_active,
        deterministic_replay=deterministic_replay,
        seed_managed=seed_managed,
        seed_version=seed_version,
    )


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
    """Actualiza un flow. El slug sigue siendo unico."""
    if slug != flow.slug and Flow.objects.filter(slug=slug).exclude(pk=flow.pk).exists():
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
    """Soft delete: marca deleted_at + is_active=false. NO borra fisicamente."""
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


def get_stats() -> dict:
    """Stats agregadas para el header del listado."""
    base = Flow.objects.filter(deleted_at__isnull=True)
    return {
        "total": base.count(),
        "active": base.filter(is_active=True).count(),
        "inactive": base.filter(is_active=False).count(),
    }


# Excepcion de dominio que core.views.MaintainerViews traduce a messages.error.
ServiceError = FlowError
