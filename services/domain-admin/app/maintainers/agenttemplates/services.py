"""Capa de negocio del mantenedor de Plantillas de Agentes.

list + signal se delegan a core.service.MaintainerService (sin reimplementar la
busqueda/paginacion ni el aggregate de la señal). El resto —validacion de
unicidad de slug (global, la tabla no tiene scope por proyecto), parseo de
capabilities, create/update/delete— vive aqui.

agent_templates NO tiene deleted_at → NO hay soft-delete: el listado parte de
AgentTemplate.objects.all() (sin filtro de borrados) y delete es HARD delete
(borra la fila). Tampoco hay toggle de status en la UI.

Las views (core.views.MaintainerViews) descubren las funciones por convencion
de nombre: entity_label="Plantilla de Agente" → attr "plantilla_de_agente",
asi que se exponen alias get_plantilla_de_agente / create_… / update_… /
delete_… que reusan las funciones de dominio (get_agenttemplate, etc.).
"""
from __future__ import annotations

from django.db import transaction

from core.service import MaintainerService

from .models import AgentTemplate



class AgentTemplateError(Exception):
    """Error de operacion sobre plantillas de agentes."""


class AgentTemplateService(MaintainerService):
    model = AgentTemplate
    search_fields = ("name", "slug", "role")
    ordering = ("name",)


_service = AgentTemplateService()


def _slug_taken(slug: str, exclude_pk=None) -> bool:
    """True si ya existe una plantilla con ese slug (unicidad global)."""
    qs = AgentTemplate.objects.filter(slug=slug)
    if exclude_pk is not None:
        qs = qs.exclude(pk=exclude_pk)
    return qs.exists()


def list_agenttemplates(search: str = "", page: int = 1, per_page: int = 20) -> dict:
    """Lista plantillas con busqueda + paginacion.

    Delega en MaintainerService.list y renombra la clave `items` ->
    `agenttemplates` para el template/tests.
    """
    data = _service.list(search=search, page=page, per_page=per_page)
    data["agenttemplates"] = data.pop("items")
    return data


def get_list_signal() -> dict:
    """Señal barata de cambios {count, version} para refresh on-change."""
    return _service.list_signal()


def get_agenttemplate(template_id: str) -> AgentTemplate:
    try:
        return AgentTemplate.objects.get(pk=template_id)
    except AgentTemplate.DoesNotExist as exc:
        raise AgentTemplateError(f"Plantilla {template_id} no existe.") from exc


@transaction.atomic
def create_agenttemplate(
    *,
    slug: str,
    name: str,
    system_prompt: str,
    personality: str = "",
    capabilities: list[str] | None = None,
    model: str = "claude-haiku-4-5",
    temperature=0.7,
    max_tokens: int = 4096,
    handoff_policy: str = "allow",
    role: str = "phase-worker",
) -> AgentTemplate:
    """Crea una plantilla. slug debe ser unico (global)."""
    if _slug_taken(slug):
        raise AgentTemplateError(f"Ya existe una plantilla con slug '{slug}'.")

    return AgentTemplate.objects.create(
        slug=slug,
        name=name,
        system_prompt=system_prompt,
        personality=personality or "",
        capabilities=capabilities or [],
        model=model,
        temperature=temperature,
        max_tokens=max_tokens,
        handoff_policy=handoff_policy,
        role=role,
    )


@transaction.atomic
def update_agenttemplate(
    template: AgentTemplate,
    *,
    slug: str,
    name: str,
    system_prompt: str,
    personality: str = "",
    capabilities: list[str] | None = None,
    model: str = "claude-haiku-4-5",
    temperature=0.7,
    max_tokens: int = 4096,
    handoff_policy: str = "allow",
    role: str = "phase-worker",
) -> AgentTemplate:
    """Actualiza una plantilla. El slug sigue siendo unico (global)."""
    if slug != template.slug and _slug_taken(slug, exclude_pk=template.pk):
        raise AgentTemplateError(f"Ya existe otra plantilla con slug '{slug}'.")

    template.slug = slug
    template.name = name
    template.system_prompt = system_prompt
    template.personality = personality or ""
    template.capabilities = capabilities or []
    template.model = model
    template.temperature = temperature
    template.max_tokens = max_tokens
    template.handoff_policy = handoff_policy
    template.role = role
    template.save()
    return template


@transaction.atomic
def delete_agenttemplate(template: AgentTemplate) -> None:
    """HARD delete: agent_templates NO tiene deleted_at, asi que se borra la
    fila fisicamente (no hay soft-delete)."""
    template.delete()





get_plantilla_de_agente = get_agenttemplate
create_plantilla_de_agente = create_agenttemplate
update_plantilla_de_agente = update_agenttemplate
delete_plantilla_de_agente = delete_agenttemplate



ServiceError = AgentTemplateError
