from __future__ import annotations

from django.db import transaction

from core.service import MaintainerService

from .models import Cron


class CronError(Exception):
    pass




class CronsService(MaintainerService):
    model = Cron
    search_fields = ("name", "slug", "cron_expression", "target_type")
    ordering = ("-created_at",)

    def base_qs(self):
        return Cron.objects.filter(deleted_at__isnull=True)


_service = CronsService()


def _filtered_cron_qs(target_types=None, enabled=None):
    qs = _service.base_qs()
    if target_types:
        qs = qs.filter(target_type__in=target_types)
    if enabled is not None:
        qs = qs.filter(enabled=enabled)
    return qs


def list_crons(search: str = "", page: int = 1, per_page: int = 20,
               target_types=None, enabled=None) -> dict:
    data = _service.list(qs=_filtered_cron_qs(target_types, enabled),
                         search=search, page=page, per_page=per_page)
    data["crons"] = data.pop("items")
    return data


def export_crons_csv(search: str = "", target_types=None, enabled=None) -> str:
    import csv
    import io
    from django.db.models import Q

    qs = _filtered_cron_qs(target_types, enabled)
    if search:

        qs = qs.filter(
            Q(name__icontains=search)
            | Q(slug__icontains=search)
            | Q(cron_expression__icontains=search)
            | Q(target_type__icontains=search)
        )
    qs = qs.distinct().order_by("name")

    buf = io.StringIO()
    w = csv.writer(buf)
    w.writerow(["Nombre", "Slug", "Expresion", "Tipo", "Target", "Habilitado", "Creado"])
    for c in qs:
        w.writerow([
            c.name, c.slug, c.cron_expression, c.get_target_type_display(),
            c.target_id, "Si" if c.enabled else "No",
            c.created_at.strftime("%Y-%m-%d %H:%M") if c.created_at else "",
        ])
    return buf.getvalue()


def get_list_signal() -> dict:
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
    from django.utils import timezone

    cron.deleted_at = timezone.now()
    cron.enabled = False
    cron.save()


@transaction.atomic
def toggle_cron_enabled(cron: Cron) -> bool:
    cron.enabled = not cron.enabled
    cron.save()
    return cron.enabled


def get_stats() -> dict:
    base = Cron.objects.filter(deleted_at__isnull=True)
    return {
        "total": base.count(),
        "enabled": base.filter(enabled=True).count(),
        "disabled": base.filter(enabled=False).count(),
    }


ServiceError = CronError
