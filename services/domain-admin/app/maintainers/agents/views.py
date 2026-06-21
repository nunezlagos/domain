"""Views del mantenedor de Agentes (migradas a core).

Las vistas estándar (list, signal, detail, create, edit, delete) las arma
core.views.MaintainerViews. Acá solo:

  1. Se configura la instancia `views` (model/form/service/templates/labels).
  2. Se sobreescriben los hooks específicos de agents:
       - do_list: usa services.list_agents (EXCLUYE soft-deleted), a
         diferencia del list genérico que listaría todo.
       - form_context / detail_context: exponen `agent_obj` (+ versiones y
         templates READ-ONLY en el detalle) que los templates ya consumen.

NO se cablea toggle: agents no alterna estado (su delete es soft-delete puro).
El guard de auth y la detección AJAX vienen de core.auth (antes _require_auth/
_is_ajax duplicados).
"""
from __future__ import annotations

from core.views import MaintainerViews

from . import services
from .forms import AgentForm
from .models import Agent


class AgentViews(MaintainerViews):
    """MaintainerViews especializado para agents (list filtrado + contextos)."""

    # --- listado: usa el service que EXCLUYE soft-deleted (el list genérico
    #     del core no filtra deleted_at). list_key="agents" ya viene seteado.
    def do_list(self, search: str, page: int) -> dict:
        return services.list_agents(search=search, page=page, per_page=self.per_page)

    # --- contextos: los templates de agents usan `agent_obj` (no `object`).
    def form_context(self, form, mode: str, instance, action: str) -> dict:
        return {
            "form": form,
            "mode": mode,
            "agent_obj": instance,
            "object": instance,
            "action": action,
        }

    def detail_context(self, instance) -> dict:
        return {
            "agent_obj": instance,
            "object": instance,
            "agent_versions": services.get_agent_versions(instance),
            "agent_templates": services.get_agent_templates(),
        }


# Instancia que cablea todo. list_key="agents" -> el template recibe la lista
# bajo `agents`. id_kwarg="agent_id" -> casa con <uuid:agent_id> de las URLs.
views = AgentViews(
    app_name="agents",
    model=Agent,
    form_class=AgentForm,
    service=services,
    templates="agents",
    search_fields=("name", "slug", "provider", "model"),
    entity_label="Agente",
    id_kwarg="agent_id",
    list_key="agents",
    per_page=20,
    search_param="q",
)
