"""Capa de negocio del mantenedor de Agentes (migrada a core).

list + signal se delegan a core.service.MaintainerService (sin reimplementar la
busqueda/paginacion ni el aggregate de la señal). La diferencia con users: el
listado EXCLUYE los soft-deleted (deleted_at != NULL), asi que se le pasa al
service un queryset base ya filtrado. El resto —slug unico, normalizacion de
skills, create/update/delete con validaciones, y los getters READ-ONLY de
versiones/templates— queda aqui.

Las views (core.views.MaintainerViews) descubren las funciones por convencion
de nombre. entity_label="Agente" -> attr "agente", por eso exponemos alias
get_agente/... apuntando a las funciones reales.
"""
from __future__ import annotations

from django.db import transaction

from core.service import MaintainerService

from .models import Agent, AgentTemplate, AgentVersion


# Error de dominio (la view lo traduce a messages.error).
class AgentError(Exception):
    """Error de operacion sobre agentes."""


# Service base reusado: list (search name/slug/provider/model + paginacion) +
# signal. El list parte de un qs que excluye soft-deleted.
class AgentsService(MaintainerService):
    model = Agent
    search_fields = ("name", "slug", "provider", "model")
    ordering = ("-created_at",)

    def base_qs(self):
        """Queryset base del listado: excluye los soft-deleted."""
        return Agent.objects.filter(deleted_at__isnull=True)


_service = AgentsService()


def _filtered_agent_qs(providers=None, statuses=None):
    """Queryset base del listado (excluye soft-deleted) filtrado por proveedor
    y/o estado (multi-select). Listas vacias = sin filtro. Se pasa como qs base a
    MaintainerService.list (que suma la busqueda)."""
    qs = _service.base_qs()
    if providers:
        qs = qs.filter(provider__in=providers)
    if statuses:
        qs = qs.filter(status__in=statuses)
    return qs


def list_agents(search: str = "", page: int = 1, per_page: int = 20,
                providers=None, statuses=None) -> dict:
    """Lista agentes (excluye soft-deleted) con busqueda + filtros + paginacion.

    Delega en MaintainerService.list pasandole el qs ya filtrado (proveedor/
    estado), y renombra la clave `items` -> `agents` para no romper el contrato
    del template/tests.
    """
    data = _service.list(
        qs=_filtered_agent_qs(providers, statuses),
        search=search, page=page, per_page=per_page,
    )
    data["agents"] = data.pop("items")
    return data


def export_agents_csv(search: str = "", providers=None, statuses=None) -> str:
    """CSV consolidado (compatible con Excel) de los agentes que matchean los
    filtros activos (proveedor/estado/busqueda). Sin paginar."""
    import csv
    import io
    from django.db.models import Q

    qs = _filtered_agent_qs(providers, statuses)
    if search:
        qs = qs.filter(
            Q(name__icontains=search) | Q(slug__icontains=search)
            | Q(provider__icontains=search) | Q(model__icontains=search)
        )
    qs = qs.distinct().order_by("name")

    buf = io.StringIO()
    w = csv.writer(buf)
    w.writerow(["Nombre", "Slug", "Proveedor", "Modelo", "Estado", "Skills", "Creado"])
    for a in qs:
        w.writerow([
            a.name, a.slug, a.provider, a.model, a.get_status_display(),
            ", ".join(a.skills_slugs or []),
            a.created_at.strftime("%Y-%m-%d %H:%M") if a.created_at else "",
        ])
    return buf.getvalue()


def list_provider_options() -> list[str]:
    """Proveedores distintos en uso (para el multi-select del filtro)."""
    return sorted(p for p in Agent.objects.values_list("provider", flat=True).distinct() if p)


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
    """Crea un agente nuevo. El slug debe ser unico (ya no hay organizacion)."""
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
    """Actualiza un agente. El slug sigue siendo unico (sin organizacion)."""
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
    """Soft delete: marca deleted_at. NO borra fisicamente."""
    from django.utils import timezone

    agent.deleted_at = timezone.now()
    agent.save()


def get_agent_versions(agent: Agent) -> list[AgentVersion]:
    """Historial de versiones del agent (READ-ONLY). Mas reciente primero."""
    return list(AgentVersion.objects.filter(agent=agent).order_by("-version"))


def get_agent_templates() -> list[AgentTemplate]:
    """Catalogo de templates de agente (READ-ONLY) para el detalle del agent."""
    return list(AgentTemplate.objects.all().order_by("name"))


def get_stats() -> dict:
    """Stats agregadas para el header del listado."""
    base = Agent.objects.filter(deleted_at__isnull=True)
    return {
        "total": base.count(),
        "seed_managed": base.filter(seed_managed=True).count(),
        "user_modified": base.filter(is_user_modified=True).count(),
    }


# --- Alias para el descubrimiento por convencion de core.views.MaintainerViews.
# entity_label="Agente" -> _entity_attr() == "agente"; core busca
# get_agente / create_agente / update_agente / delete_agente. Apuntamos esos
# nombres a las funciones reales. (NO hay toggle: agents no alterna estado.)
get_agente = get_agent
create_agente = create_agent
update_agente = update_agent
delete_agente = delete_agent
ServiceError = AgentError
