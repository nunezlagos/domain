from __future__ import annotations

from django.db import transaction

from core.service import MaintainerService

from .models import Flow, FlowVersion


class FlowError(Exception):
    pass


class FlowService(MaintainerService):
    model = Flow
    search_fields = ("name", "slug", "description")
    ordering = ("-created_at",)


_service = FlowService()


def _filtered_flow_qs(is_active=None):
    qs = Flow.objects.filter(deleted_at__isnull=True)
    if is_active is not None:
        qs = qs.filter(is_active=is_active)
    return qs


def list_flows(search: str = "", page: int = 1, per_page: int = 20,
               is_active=None) -> dict:
    data = _service.list(qs=_filtered_flow_qs(is_active),
                         search=search, page=page, per_page=per_page)
    data["flows"] = data.pop("items")
    return data


def export_flows_csv(search: str = "", is_active=None) -> str:
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
    return _service.list_signal()


def get_flow(flow_id: str) -> Flow:
    try:
        return Flow.objects.get(pk=flow_id)
    except Flow.DoesNotExist as exc:
        raise FlowError(f"Flow {flow_id} no existe.") from exc


def get_flow_versions(flow: Flow) -> list[FlowVersion]:
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
    from django.utils import timezone

    flow.deleted_at = timezone.now()
    flow.is_active = False
    flow.save()


@transaction.atomic
def toggle_flow_status(flow: Flow) -> bool:
    flow.is_active = not flow.is_active
    flow.save()
    return flow.is_active


def get_stats() -> dict:
    base = Flow.objects.filter(deleted_at__isnull=True)
    return {
        "total": base.count(),
        "active": base.filter(is_active=True).count(),
        "inactive": base.filter(is_active=False).count(),
    }


ServiceError = FlowError
