"""Views del mantenedor de Agentes (migradas a core).

Las vistas estandar (list, signal, detail, create, edit, delete) las arma
core.views.MaintainerViews. Aqui solo:

  1. Se configura la instancia `views` (model/form/service/templates/labels).
  2. Se sobreescriben los hooks especificos de agents:
       - do_list: usa services.list_agents (EXCLUYE soft-deleted), a
         diferencia del list generico que listaria todo.
       - form_context / detail_context: exponen `agent_obj` (+ versiones y
         templates READ-ONLY en el detalle) que los templates ya consumen.

NO se cablea toggle: agents no alterna estado (su delete es soft-delete puro).
El guard de auth y la deteccion AJAX vienen de core.auth (antes _require_auth/
_is_ajax duplicados).
"""
from __future__ import annotations

from django.http import HttpResponse

from core.auth import require_auth
from core.views import MaintainerViews

from . import services
from .forms import AgentForm
from .models import Agent


class AgentViews(MaintainerViews):
    """MaintainerViews especializado para agents (list filtrado + contextos)."""

    # --- list con filtros (proveedor multi-select / estado). Guardamos el
    #     request para que do_list/list_context lean los GET; el resto lo arma core.
    def list(self, request):
        self._list_request = request
        return super().list(request)

    # --- listado: usa el service que EXCLUYE soft-deleted (el list generico
    #     del core no filtra deleted_at) + aplica los filtros activos.
    #     list_key="agents" ya viene seteado.
    def do_list(self, search: str, page: int) -> dict:
        req = getattr(self, "_list_request", None)
        providers = req.GET.getlist("provider") if req else []
        status = req.GET.get("status") if req else ""
        return services.list_agents(
            search=search, page=page, per_page=self.per_page,
            providers=providers, statuses=[status] if status else None,
        )

    # --- contextos: los templates de agents usan `agent_obj` (no `object`).
    def list_context(self, data: dict, search: str) -> dict:
        ctx = super().list_context(data, search)
        req = getattr(self, "_list_request", None)
        # Opciones + seleccion actual para el container de filtros.
        ctx["provider_options"] = services.list_provider_options()
        ctx["status_options"] = Agent.STATUS_CHOICES
        ctx["selected_providers"] = req.GET.getlist("provider") if req else []
        ctx["selected_status"] = req.GET.get("status") if req else ""
        return ctx

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


def export_agents(request):
    """Export CSV (consolidado, abre en Excel) de los agentes filtrados.
    Respeta los filtros activos: q (busqueda), provider[] (multi-select), status."""
    if (redir := require_auth(request)):
        return redir
    status = request.GET.get("status") or ""
    csv_data = services.export_agents_csv(
        search=(request.GET.get("q") or "").strip(),
        providers=request.GET.getlist("provider"),
        statuses=[status] if status else None,
    )
    resp = HttpResponse(csv_data, content_type="text/csv; charset=utf-8")
    resp["Content-Disposition"] = 'attachment; filename="agentes.csv"'
    return resp
