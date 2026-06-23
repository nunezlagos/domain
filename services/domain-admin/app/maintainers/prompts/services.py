"""Capa de negocio del mantenedor de Prompts (migrada a core).

list + signal se delegan a core.service.MaintainerService (sin reimplementar la
busqueda/paginacion ni el aggregate de la señal). Lo propio del dominio —unicidad
de la tripleta (project_id, slug, version), tags, toggle de is_active que reactiva
soft-deleted— sigue aqui.

El listado excluye los soft-deleted (deleted_at != NULL): por eso PromptService
sobreescribe `list` para inyectar ese filtro en el queryset base antes de
delegar en el MaintainerService.

Las views (core.views.MaintainerViews) descubren las funciones por convencion de
nombre: get_prompt / create_prompt / update_prompt / delete_prompt /
toggle_prompt_status / get_list_signal. entity_label="Prompt" -> attr "prompt",
asi que esos nombres ya calzan (no hacen falta alias).
"""
from __future__ import annotations

from django.db import transaction
from django.utils import timezone

from core.service import MaintainerService

from .models import Prompt


# Error de dominio (la view lo traduce a messages.error).
class PromptError(Exception):
    """Error de operacion sobre prompts."""


class PromptService(MaintainerService):
    """list (search slug/description/body) + signal, excluyendo soft-deleted."""

    model = Prompt
    search_fields = ("slug", "description", "body")
    ordering = ("-created_at",)

    def list(self, *args, **kwargs):
        # El listado del mantenedor NO muestra soft-deleted.
        kwargs.setdefault("qs", Prompt.objects.filter(deleted_at__isnull=True))
        return super().list(*args, **kwargs)


_service = PromptService()


def _filtered_prompt_qs(is_active=None):
    """Queryset base de Prompt para el listado/export: replica la exclusion de
    soft-deleted que hace PromptService.list (deleted_at IS NULL) y suma el
    filtro opcional is_active. None = sin filtro de estado.

    Se construye el qs base aqui (en vez de delegar el default de
    PromptService.list) porque al pasar un `qs` explicito a list, el setdefault
    de PromptService.list no aplica; asi garantizamos que los soft-deleted
    sigan excluidos."""
    qs = Prompt.objects.filter(deleted_at__isnull=True)
    if is_active is not None:
        qs = qs.filter(is_active=is_active)
    return qs


def list_prompts(search: str = "", page: int = 1, per_page: int = 20,
                 is_active=None) -> dict:
    """Lista prompts (no soft-deleted) con busqueda + filtro is_active + paginacion.

    Delega en MaintainerService.list (pasando el qs ya filtrado) y renombra la
    clave `items` -> `prompts` para no romper el contrato del template/tests.
    """
    data = _service.list(qs=_filtered_prompt_qs(is_active),
                         search=search, page=page, per_page=per_page)
    data["prompts"] = data.pop("items")
    return data


def export_prompts_csv(search: str = "", is_active=None) -> str:
    """CSV consolidado (compatible con Excel) de los prompts que matchean los
    filtros activos (estado/busqueda). Sin paginar."""
    import csv
    import io
    from django.db.models import Q

    qs = _filtered_prompt_qs(is_active)
    if search:
        qs = qs.filter(
            Q(slug__icontains=search)
            | Q(description__icontains=search)
            | Q(body__icontains=search)
        )
    qs = qs.distinct().order_by("slug")

    buf = io.StringIO()
    w = csv.writer(buf)
    w.writerow(["Slug", "Descripcion", "Version", "Activo (Si/No)", "Creado"])
    for p in qs:
        w.writerow([
            p.slug,
            p.description,
            p.version,
            "Si" if p.is_active else "No",
            p.created_at.strftime("%Y-%m-%d %H:%M") if p.created_at else "",
        ])
    return buf.getvalue()


def get_list_signal() -> dict:
    """Señal barata de cambios {count, version} para refresh on-change."""
    return _service.list_signal()


def get_prompt(prompt_id: str) -> Prompt:
    try:
        return Prompt.objects.get(pk=prompt_id)
    except Prompt.DoesNotExist as exc:
        raise PromptError(f"Prompt {prompt_id} no existe.") from exc


def _slug_taken(project_id, slug: str, version: int, exclude_pk=None) -> bool:
    """La unicidad real es (project_id, slug, version)."""
    qs = Prompt.objects.filter(project_id=project_id, slug=slug, version=version)
    if exclude_pk is not None:
        qs = qs.exclude(pk=exclude_pk)
    return qs.exists()


@transaction.atomic
def create_prompt(
    *,
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
    """Crea un prompt nuevo. (project_id, slug, version) debe ser unica."""
    if _slug_taken(project_id, slug, version):
        raise PromptError(
            f"Ya existe un prompt con slug '{slug}' v{version} en este proyecto."
        )

    return Prompt.objects.create(
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

    project_id no se edita (define el contexto de unicidad). La tripleta
    (project, slug, version) sigue siendo unica, excluyendo el propio registro.
    """
    if _slug_taken(prompt.project_id, slug, version, exclude_pk=prompt.pk):
        raise PromptError(
            f"Ya existe otro prompt con slug '{slug}' v{version} en este proyecto."
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
    """Soft delete: marca deleted_at + is_active=False. NO borra fisicamente."""
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


def get_stats() -> dict:
    """Stats agregadas para el header del listado."""
    base = Prompt.objects.filter(deleted_at__isnull=True)
    return {
        "total": base.count(),
        "active": base.filter(is_active=True).count(),
        "inactive": base.filter(is_active=False).count(),
    }


# Error de dominio que core.views.MaintainerViews busca como `ServiceError`.
ServiceError = PromptError
