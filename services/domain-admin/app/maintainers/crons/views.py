from __future__ import annotations

from django.http import HttpResponse

from core.auth import require_auth
from core.views import MaintainerViews

from . import services
from .forms import CronForm
from .models import Cron


class CronViews(MaintainerViews):




    def _form_payload(self, form) -> dict:
        return {
            "name": form.cleaned_data["name"],
            "slug": form.cleaned_data["slug"],
            "description": form.cleaned_data["description"],
            "cron_expression": form.cleaned_data["cron_expression"],
            "timezone": form.cleaned_data["timezone"],
            "target_type": form.cleaned_data["target_type"],
            "target_id": form.cleaned_data["target_id"],
            "inputs": form.cleaned_data["inputs"],
            "enabled": form.cleaned_data["enabled"],
        }



    def list(self, request):
        self._list_request = request
        return super().list(request)




    def do_list(self, search: str, page: int) -> dict:
        req = getattr(self, "_list_request", None)
        target_types = req.GET.getlist("target_type") if req else []

        val = req.GET.get("enabled") if req else None
        enabled = None if not val else (val == "1")
        return services.list_crons(
            search=search, page=page, per_page=self.per_page,
            target_types=target_types, enabled=enabled,
        )



    def do_toggle(self, instance) -> str:
        enabled = services.toggle_cron_enabled(instance)
        return "habilitado" if enabled else "deshabilitado"


    def do_delete(self, instance) -> None:
        services.delete_cron(instance)


    def list_context(self, data: dict, search: str) -> dict:
        ctx = super().list_context(data, search)
        ctx["page_title"] = "Crons"
        req = getattr(self, "_list_request", None)

        ctx["target_type_options"] = Cron.TARGET_TYPE_CHOICES
        ctx["selected_target_type"] = req.GET.getlist("target_type") if req else []
        ctx["selected_enabled"] = (req.GET.get("enabled") or "") if req else ""
        return ctx

    def form_context(self, form, mode: str, instance, action: str) -> dict:
        return {
            "form": form,
            "mode": mode,
            "cron_obj": instance,
            "object": instance,
            "action": action,
        }

    def detail_context(self, instance) -> dict:
        return {"cron_obj": instance, "object": instance}




views = CronViews(
    app_name="crons",
    model=Cron,
    form_class=CronForm,
    service=services,
    templates="crons",
    search_fields=("name", "slug", "cron_expression", "target_type"),
    entity_label="Cron",
    id_kwarg="cron_id",
    list_key="crons",
    per_page=10,
    search_param="q",
)


def export_crons(request):
    if (redir := require_auth(request)):
        return redir
    val = request.GET.get("enabled")
    enabled = None if not val else (val == "1")
    csv_data = services.export_crons_csv(
        search=(request.GET.get("q") or "").strip(),
        target_types=request.GET.getlist("target_type"),
        enabled=enabled,
    )
    resp = HttpResponse(csv_data, content_type="text/csv; charset=utf-8")
    resp["Content-Disposition"] = 'attachment; filename="crons.csv"'
    return resp
