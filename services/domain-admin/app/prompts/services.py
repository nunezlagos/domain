"""Capa de negocio del mantenedor de Prompts.

Patrón: las views solo hacen HTTP request/response; toda la lógica de
modelo vive acá. Esto facilita testing unitario sin tocar HTTP.

La tabla `prompts` la administra domain-mcp (managed=False); Django solo
lee/escribe vía ORM. Soft-delete (deleted_at) + toggle de habilitación
(is_active bool). La unicidad real es (organization_id, project_id, slug,
version).
"""
from __future__ import annotations

from django.db import transaction
from django.db.models import Count, Max
from django.utils import timezone

from .models import Prompt


# Error de dominio (la view lo traduce a messages.error).
class PromptError(Exception):
    """Error de operación sobre prompts."""


def list_prompts(search: str = "", page: int = 1, per_page: int = 20) -> dict:
    """Lista prompts con búsqueda opcional + paginación.

    Excluye los soft-deleted (deleted_at != NULL). Búsqueda sobre
    slug / description / body.

    Retorna dict con: prompts, total, page, per_page, total_pages,
    has_next, has_prev.
    """
    qs = Prompt.objects.filter(deleted_at__isnull=True)
    if search:
        qs = (
            qs.filter(slug__icontains=search)
            | qs.filter(description__icontains=search)
            | qs.filter(body__icontains=search)
        )
    qs = qs.distinct().order_by("-created_at")

    total = qs.count()
    total_pages = max(1, (total + per_page - 1) // per_page)
    start = (page - 1) * per_page
    end = start + per_page
    prompts = list(qs[start:end])

    return {
        "prompts": prompts,
        "total": total,
        "page": page,
        "per_page": per_page,
        "total_pages": total_pages,
        "has_next": end < total,
        "has_prev": page > 1,
    }


def get_prompt(prompt_id: str) -> Prompt:
    try:
        return Prompt.objects.get(pk=prompt_id)
    except Prompt.DoesNotExist as exc:
        raise PromptError(f"Prompt {prompt_id} no existe.") from exc


def _slug_taken(
    organization_id, project_id, slug: str, version: int, exclude_pk=None
) -> bool:
    """La unicidad real es (organization_id, project_id, slug, version)."""
    qs = Prompt.objects.filter(
        organization_id=organization_id,
        project_id=project_id,
        slug=slug,
        version=version,
    )
    if exclude_pk is not None:
        qs = qs.exclude(pk=exclude_pk)
    return qs.exists()


@transaction.atomic
def create_prompt(
    *,
    organization_id,
    slug: str,
    body: str,
    version: int = 1,
    project_id=None,
    created_by=None,
    description: str = "",
    is_active: bool = True,
    variables=None,
    tags=None,
) -> Prompt:
    """Crea un prompt nuevo.

    La combinación (organization_id, project_id, slug, version) debe ser única.
    """
    if _slug_taken(organization_id, project_id, slug, version):
        raise PromptError(
            f"Ya existe un prompt con slug '{slug}' v{version} en este "
            f"contexto (organización/proyecto)."
        )

    prompt = Prompt.objects.create(
        organization_id=organization_id,
        project_id=project_id,
        created_by=created_by,
        slug=slug,
        version=version,
        body=body,
        description=description or "",
        is_active=is_active,
        variables=variables if variables is not None else [],
        tags=tags if tags is not None else [],
    )
    return prompt


@transaction.atomic
def update_prompt(
    prompt: Prompt,
    *,
    slug: str,
    body: str,
    version: int,
    description: str = "",
    is_active: bool = True,
    variables=None,
    tags=None,
) -> Prompt:
    """Actualiza un prompt.

    organization_id / project_id no se editan (definen el contexto de
    unicidad). La cuádrupla (org, project, slug, version) sigue siendo única,
    excluyendo el propio registro.
    """
    if _slug_taken(
        prompt.organization_id,
        prompt.project_id,
        slug,
        version,
        exclude_pk=prompt.pk,
    ):
        raise PromptError(
            f"Ya existe otro prompt con slug '{slug}' v{version} en este "
            f"contexto (organización/proyecto)."
        )

    prompt.slug = slug
    prompt.body = body
    prompt.version = version
    prompt.description = description or ""
    prompt.is_active = is_active
    if variables is not None:
        prompt.variables = variables
    if tags is not None:
        prompt.tags = tags
    prompt.save()
    return prompt


@transaction.atomic
def delete_prompt(prompt: Prompt) -> None:
    """Soft delete: marca deleted_at + is_active=False. NO borra físicamente."""
    prompt.deleted_at = timezone.now()
    prompt.is_active = False
    prompt.save()


@transaction.atomic
def toggle_prompt_status(prompt: Prompt) -> bool:
    """Alterna is_active. Retorna el nuevo valor de is_active.

    Un prompt soft-deleted que se reactiva limpia deleted_at (vuelve a quedar
    visible y habilitado).
    """
    if prompt.is_active:
        prompt.is_active = False
    else:
        prompt.is_active = True
        prompt.deleted_at = None
    prompt.save()
    return prompt.is_active


def get_list_signal() -> dict:
    """Señal barata de cambios para refresh on-change.

    NO es polling ciego de la tabla. Devuelve count + max(updated_at):
    cualquier alta, edición, baja (soft) o toggle muta uno de los dos
    (updated_at lo bumpea el trigger set_updated_at en la BD; created_at
    de altas nuevas sube el max). El front compara contra su última señal
    y solo re-renderiza la tabla cuando algo cambió en la BD — incluyendo
    inserts de otros servicios (domain-mcp) que escriben directo en `prompts`.

    Query barata: SELECT count(*), max(updated_at) FROM prompts.
    """
    agg = Prompt.objects.aggregate(total=Count("id"), latest=Max("updated_at"))
    latest = agg["latest"]
    return {
        "count": agg["total"] or 0,
        "version": latest.isoformat() if latest else "",
    }


def get_stats() -> dict:
    """Stats agregadas para el header del listado."""
    base = Prompt.objects.filter(deleted_at__isnull=True)
    return {
        "total": base.count(),
        "active": base.filter(is_active=True).count(),
        "inactive": base.filter(is_active=False).count(),
    }
