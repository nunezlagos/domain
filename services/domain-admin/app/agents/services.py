"""Capa de negocio del mantenedor de Agentes.

Patrón: las views solo hacen HTTP request/response; toda la lógica de
modelo vive acá. Esto facilita testing unitario sin tocar HTTP.

La tabla `agents` la administra domain-mcp (managed=False); Django solo
lee/escribe vía ORM.

Soft-delete: SÍ (la tabla tiene deleted_at). El delete marca deleted_at;
NO se setea un status terminal porque la tabla agents NO tiene columna
status.

Toggle de estado: NO aplica (no hay columna status con estados alternables).

agent_versions y agent_templates son READ-ONLY (se exponen vía getters
para el detalle del agent; no tienen CRUD por modal).
"""
from __future__ import annotations

from django.db import transaction

from .models import Agent, AgentTemplate, AgentVersion


# Error de dominio (la view lo traduce a messages.error).
class AgentError(Exception):
    """Error de operación sobre agentes."""


def list_agents(search: str = "", page: int = 1, per_page: int = 20) -> dict:
    """Lista agentes con búsqueda opcional + paginación.

    Excluye los soft-deleted (deleted_at != NULL). Búsqueda sobre
    name / slug / provider / model.

    Retorna dict con: agents, total, page, per_page, total_pages,
    has_next, has_prev.
    """
    qs = Agent.objects.filter(deleted_at__isnull=True)
    if search:
        qs = (
            qs.filter(name__icontains=search)
            | qs.filter(slug__icontains=search)
            | qs.filter(provider__icontains=search)
            | qs.filter(model__icontains=search)
        )
    qs = qs.distinct().order_by("-created_at")

    total = qs.count()
    total_pages = max(1, (total + per_page - 1) // per_page)
    start = (page - 1) * per_page
    end = start + per_page
    agents = list(qs[start:end])

    return {
        "agents": agents,
        "total": total,
        "page": page,
        "per_page": per_page,
        "total_pages": total_pages,
        "has_next": end < total,
        "has_prev": page > 1,
    }


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

    agent = Agent.objects.create(
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
    return agent


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
    """Soft delete: marca deleted_at. NO borra físicamente.

    La tabla agents no tiene columna status, así que el soft-delete solo
    setea deleted_at (no hay status terminal que marcar).
    """
    from django.utils import timezone

    agent.deleted_at = timezone.now()
    agent.save()


def get_agent_versions(agent: Agent) -> list[AgentVersion]:
    """Historial de versiones del agent (READ-ONLY). Más reciente primero."""
    return list(
        AgentVersion.objects.filter(agent=agent).order_by("-version")
    )


def get_agent_templates() -> list[AgentTemplate]:
    """Catálogo de templates de agente (READ-ONLY) para referencia en el detalle."""
    return list(AgentTemplate.objects.all().order_by("name"))


def get_list_signal() -> dict:
    """Señal barata de cambios para refresh on-change.

    NO es polling ciego de la tabla. Devuelve count + max(updated_at):
    cualquier alta, edición o baja (soft) muta uno de los dos (updated_at
    lo bumpea el trigger set_updated_at en la BD; created_at de altas
    nuevas sube el max). El front compara contra su última señal y solo
    re-renderiza la tabla cuando algo cambió en la BD — incluyendo inserts
    de otros servicios (domain-mcp) que escriben directo en `agents`.

    Query barata: SELECT count(*), max(updated_at) FROM agents.
    """
    from django.db.models import Count, Max

    agg = Agent.objects.aggregate(total=Count("id"), latest=Max("updated_at"))
    latest = agg["latest"]
    return {
        "count": agg["total"] or 0,
        "version": latest.isoformat() if latest else "",
    }


def get_stats() -> dict:
    """Stats agregadas para el header del listado."""
    base = Agent.objects.filter(deleted_at__isnull=True)
    return {
        "total": base.count(),
        "seed_managed": base.filter(seed_managed=True).count(),
        "user_modified": base.filter(is_user_modified=True).count(),
    }
