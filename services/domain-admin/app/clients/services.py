"""Capa de negocio del mantenedor de Clientes (mandantes).

Patrón: las views solo hacen HTTP request/response; toda la lógica de
modelo vive acá. Esto facilita testing unitario sin tocar HTTP.

La tabla `clients` la administra domain-mcp (managed=False); Django solo
lee/escribe vía ORM. Soft-delete + toggle de status (active/inactive/archived).
"""
from __future__ import annotations

from django.db import transaction

from .models import Client


# Error de dominio (la view lo traduce a messages.error).
class ClientError(Exception):
    """Error de operación sobre clientes."""


def list_clients(search: str = "", page: int = 1, per_page: int = 20) -> dict:
    """Lista clientes con búsqueda opcional + paginación.

    Excluye los soft-deleted (deleted_at != NULL). Búsqueda sobre
    name / slug / tax_id / contact_email.

    Retorna dict con: clients, total, page, per_page, total_pages,
    has_next, has_prev.
    """
    qs = Client.objects.filter(deleted_at__isnull=True)
    if search:
        qs = (
            qs.filter(name__icontains=search)
            | qs.filter(slug__icontains=search)
            | qs.filter(tax_id__icontains=search)
            | qs.filter(contact_email__icontains=search)
        )
    qs = qs.distinct().order_by("-created_at")

    total = qs.count()
    total_pages = max(1, (total + per_page - 1) // per_page)
    start = (page - 1) * per_page
    end = start + per_page
    clients = list(qs[start:end])

    return {
        "clients": clients,
        "total": total,
        "page": page,
        "per_page": per_page,
        "total_pages": total_pages,
        "has_next": end < total,
        "has_prev": page > 1,
    }


def get_client(client_id: str) -> Client:
    try:
        return Client.objects.get(pk=client_id)
    except Client.DoesNotExist as exc:
        raise ClientError(f"Cliente {client_id} no existe.") from exc


@transaction.atomic
def create_client(
    *,
    organization_id: str,
    name: str,
    slug: str,
    tax_id: str = "",
    contact_email: str = "",
    contact_phone: str = "",
    address: str = "",
    status: str = "active",
) -> Client:
    """Crea un cliente nuevo. slug debe ser único dentro de la organización."""
    if Client.objects.filter(organization_id=organization_id, slug=slug).exists():
        raise ClientError(
            f"Ya existe un cliente con slug '{slug}' en esta organización."
        )

    client = Client.objects.create(
        organization_id=organization_id,
        name=name,
        slug=slug,
        tax_id=tax_id or "",
        contact_email=contact_email or "",
        contact_phone=contact_phone or "",
        address=address or "",
        status=status,
    )
    return client


@transaction.atomic
def update_client(
    client: Client,
    *,
    name: str,
    slug: str,
    tax_id: str = "",
    contact_email: str = "",
    contact_phone: str = "",
    address: str = "",
    status: str = "active",
) -> Client:
    """Actualiza un cliente. El slug sigue siendo único per-organización."""
    if slug != client.slug and Client.objects.filter(
        organization_id=client.organization_id, slug=slug
    ).exclude(pk=client.pk).exists():
        raise ClientError(
            f"Ya existe otro cliente con slug '{slug}' en esta organización."
        )

    client.name = name
    client.slug = slug
    client.tax_id = tax_id or ""
    client.contact_email = contact_email or ""
    client.contact_phone = contact_phone or ""
    client.address = address or ""
    client.status = status
    client.save()
    return client


@transaction.atomic
def delete_client(client: Client) -> None:
    """Soft delete: marca deleted_at + status=archived. NO borra físicamente."""
    from django.utils import timezone

    client.deleted_at = timezone.now()
    client.status = "archived"
    client.save()


@transaction.atomic
def toggle_client_status(client: Client) -> str:
    """Alterna active <-> inactive. Retorna el nuevo status.

    Un cliente archivado vuelve a active al togglear (reactivación).
    """
    if client.status == "active":
        client.status = "inactive"
    else:
        # inactive o archived -> active
        client.status = "active"
    client.save()
    return client.status


def get_list_signal() -> dict:
    """Señal barata de cambios para refresh on-change.

    NO es polling ciego de la tabla. Devuelve count + max(updated_at):
    cualquier alta, edición, baja (soft) o toggle muta uno de los dos
    (updated_at lo bumpea el trigger set_updated_at en la BD; created_at
    de altas nuevas sube el max). El front compara contra su última señal
    y solo re-renderiza la tabla cuando algo cambió en la BD — incluyendo
    inserts de otros servicios (domain-mcp) que escriben directo en `clients`.

    Query barata: SELECT count(*), max(updated_at) FROM clients.
    """
    from django.db.models import Count, Max

    agg = Client.objects.aggregate(total=Count("id"), latest=Max("updated_at"))
    latest = agg["latest"]
    return {
        "count": agg["total"] or 0,
        "version": latest.isoformat() if latest else "",
    }


def get_stats() -> dict:
    """Stats agregadas para el header del listado."""
    base = Client.objects.filter(deleted_at__isnull=True)
    return {
        "total": base.count(),
        "active": base.filter(status="active").count(),
        "inactive": base.filter(status="inactive").count(),
    }
