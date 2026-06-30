from __future__ import annotations

from django.http import HttpResponse

from core.auth import require_auth
from core.views import MaintainerViews

from . import services
from .forms import AgentForm
from .models import Agent

class AgentViews(MaintainerViews):
    def list(self, request):
        self._list_request = request
        return super().list(request)

    def do_list(self, search: str, page: int) -> dict:
        req = getattr(self, "_list_request", None)
        providers = req.GET.getlist("provider") if req else []
        status = req.GET.get("status") if req else ""
        return services.list_agents(
            search=search, page=page, per_page=self.per_page,
            providers=providers, statuses=[status] if status else None,
        )


    def list_context(self, data: dict, search: str) -> dict:
        ctx = super().list_context(data, search)
        req = getattr(self, "_list_request", None)

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
    per_page=10,
    search_param="q",
)


def export_agents(request):
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
