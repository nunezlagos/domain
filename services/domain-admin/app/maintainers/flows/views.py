from __future__ import annotations

import json

from django.contrib import messages
from django.http import HttpResponse, HttpResponseRedirect

from core.auth import require_auth
from core.views import MaintainerViews

from . import services
from .forms import FlowForm
from .models import Flow


def _pretty_spec(flow) -> str:
    spec = getattr(flow, "spec", None)
    if spec in (None, ""):
        return ""
    if isinstance(spec, (dict, list)):
        return json.dumps(spec, indent=2, ensure_ascii=False)
    try:
        return json.dumps(json.loads(spec), indent=2, ensure_ascii=False)
    except (ValueError, TypeError):
        return str(spec)


class FlowViews(MaintainerViews):



    def list(self, request):
        self._list_request = request
        return super().list(request)




    def create(self, request):
        if (redir := require_auth(request)):
            return redir
        messages.info(
            request,
            "Los flows no se crean desde el dashboard: el catalogo lo gestiona la plataforma.",
        )
        return HttpResponseRedirect(self.url("list"))





    def do_list(self, search: str, page: int) -> dict:
        req = getattr(self, "_list_request", None)
        val = req.GET.get("active") if req else None
        is_active = None if not val else (val == "1")
        return services.list_flows(
            search=search, page=page, per_page=self.per_page,
            is_active=is_active,
        )



    def list_context(self, data: dict, search: str) -> dict:
        ctx = super().list_context(data, search)
        req = getattr(self, "_list_request", None)
        ctx["selected_active"] = req.GET.get("active", "") if req else ""
        return ctx


    def form_context(self, form, mode: str, instance, action: str) -> dict:
        ctx = {
            "form": form,
            "mode": mode,
            "flow_obj": instance,
            "object": instance,
            "action": action,
        }

        if mode == "edit" and instance is not None:
            ctx["flow_versions"] = services.get_flow_versions(instance)
        return ctx

    def detail_context(self, instance) -> dict:
        return {
            "flow_obj": instance,
            "object": instance,
            "flow_versions": services.get_flow_versions(instance),
            "spec_pretty": _pretty_spec(instance),
        }




views = FlowViews(
    app_name="flows",
    model=Flow,
    form_class=FlowForm,
    service=services,
    templates="flows",
    search_fields=("name", "slug", "description"),
    entity_label="Flow",
    id_kwarg="flow_id",
    list_key="flows",
    per_page=10,
    search_param="q",
)


def export_flows(request):
    if (redir := require_auth(request)):
        return redir
    val = request.GET.get("active") or ""
    is_active = None if not val else (val == "1")
    csv_data = services.export_flows_csv(
        search=(request.GET.get("q") or "").strip(),
        is_active=is_active,
    )
    resp = HttpResponse(csv_data, content_type="text/csv; charset=utf-8")
    resp["Content-Disposition"] = 'attachment; filename="flows.csv"'
    return resp
