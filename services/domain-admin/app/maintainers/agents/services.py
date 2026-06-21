"""Capa de negocio del mantenedor de Agentes (migrada a core).

list + signal se delegan a core.service.MaintainerService (sin reimplementar la
búsqueda/paginación ni el aggregate de la señal). La diferencia con users: el
listado EXCLUYE los soft-deleted (deleted_at != NULL), así que se le pasa al
service un queryset base ya filtrado. El resto —slug único, normalización de
skills, create/update/delete con validaciones, y los getters READ-ONLY de
versiones/templates— queda acá.

Las views (core.views.MaintainerViews) descubren las funciones por convención
de nombre. entity_label="Agente" -> attr "agente", por eso exponemos alias
get_agente/... apuntando a las funciones reales.
"""
from __future__ import annotations

from django.db import transaction

from core.service import MaintainerService

from .models import Agent, AgentTemplate, AgentVersion


# Error de dominio (la view lo traduce a messages.error).
class AgentError(Exception):
    """Error de operación sobre agentes."""


# Service base reusado: list (search name/slug/provider/model + paginación) +
# signal. El list parte de un qs que excluye soft-deleted.
class AgentsService(MaintainerService):
    model = Agent
    search_fields = ("name", "slug", "provider", "model")
    ordering = ("-created_at",)

    def base_qs(self):
        """Queryset base del listado: excluye los soft-deleted."""
        return Agent.objects.filter(deleted_at__isnull=True)


_service = AgentsService()


def list_agents(search: str = "", page: int = 1, per_page: int = 20) -> dict:
    """Lista agentes (excluye soft-deleted) con búsqueda + paginación.

    Delega en MaintainerService.list pasándole el qs ya filtrado, y renombra la
    clave `items` -> `agents` para no romper el contrato del template/tests.
    """
    data = _service.list(
        qs=_service.base_qs(), search=search, page=page, per_page=per_page
    )
    data["agents"] = data.pop("items")
    return data


def get_list_signal() -> dict:
    """Señal barata de cambios {count, version} para refresh on-change."""
    return _service.list_signal()


def get_agent(agent_id: str) -> Agent:
    try:
        return Agent.objects.get(pk=agent_id)
    except Agent.DoesNotExist as exc:
        raise AgentError(f"Agente {agent_id} no existe.") from exc


@transaction.atomic
def create_agent(
    *,
    name: str,
    slug: str,
    provider: str,
    model: str,
    description: str = "",
    system_prompt: str = "",
    skills_slugs: list[str] | None = None,
    max_iterations: int = 20,
    token_budget: int | None = None,
    temperature=None,
) -> Agent:
    """Crea un agente nuevo. El slug debe ser único (ya no hay organización)."""
    if Agent.objects.filter(slug=slug).exists():
        raise AgentError(f"Ya existe un agente con slug '{slug}'.")

    return Agent.objects.create(
        name=name,
        slug=slug,
        provider=provider,
        model=model,
        description=description or "",
        system_prompt=system_prompt or "",
        skills_slugs=skills_slugs or [],
        max_iterations=max_iterations,
        token_budget=token_budget,
        temperature=temperature,
    )


@transaction.atomic
def update_agent(
    agent: Agent,
    *,
    name: str,
    slug: str,
    provider: str,
    model: str,
    description: str = "",
    system_prompt: str = "",
    skills_slugs: list[str] | None = None,
    max_iterations: int = 20,
    token_budget: int | None = None,
    temperature=None,
) -> Agent:
    """Actualiza un agente. El slug sigue siendo único (sin organización)."""
    if slug != agent.slug and Agent.objects.filter(
        slug=slug
    ).exclude(pk=agent.pk).exists():
        raise AgentError(f"Ya existe otro agente con slug '{slug}'.")

    agent.name = name
    agent.slug = slug
    agent.provider = provider
    agent.model = model
    agent.description = description or ""
    agent.system_prompt = system_prompt or ""
    agent.skills_slugs = skills_slugs or []
    agent.max_iterations = max_iterations
    agent.token_budget = token_budget
    agent.temperature = temperature
    agent.save()
    return agent


@transaction.atomic
def delete_agent(agent: Agent) -> None:
    """Soft delete: marca deleted_at. NO borra físicamente."""
    from django.utils import timezone

    agent.deleted_at = timezone.now()
    agent.save()


def get_agent_versions(agent: Agent) -> list[AgentVersion]:
    """Historial de versiones del agent (READ-ONLY). Más reciente primero."""
    return list(AgentVersion.objects.filter(agent=agent).order_by("-version"))


def get_agent_templates() -> list[AgentTemplate]:
    """Catálogo de templates de agente (READ-ONLY) para el detalle del agent."""
    return list(AgentTemplate.objects.all().order_by("name"))


def get_stats() -> dict:
    """Stats agregadas para el header del listado."""
    base = Agent.objects.filter(deleted_at__isnull=True)
    return {
        "total": base.count(),
        "seed_managed": base.filter(seed_managed=True).count(),
        "user_modified": base.filter(is_user_modified=True).count(),
    }


# --- Alias para el descubrimiento por convención de core.views.MaintainerViews.
# entity_label="Agente" -> _entity_attr() == "agente"; core busca
# get_agente / create_agente / update_agente / delete_agente. Apuntamos esos
# nombres a las funciones reales. (NO hay toggle: agents no alterna estado.)
get_agente = get_agent
create_agente = create_agent
update_agente = update_agent
delete_agente = delete_agent
ServiceError = AgentError
