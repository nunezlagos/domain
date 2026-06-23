"""Views del mantenedor de Crons (schedules), migradas a core.

Las 7 vistas estandar (list, signal, detail, create, edit, delete, toggle) las
arma core.views.MaintainerViews. Aqui solo:

  1. Se configura la instancia `views` (model/form/service/templates/labels).
  2. Se sobreescriben los hooks especificos de crons:
       - _form_payload: pasa los campos del form tal cual al service.
       - do_toggle: la dimension alternable es el flag booleano `enabled`,
         NO `status` (el toggle generico del core alterna status).
       - do_delete: soft delete propio (deleted_at + enabled=False).
       - form_context / detail_context: exponen `cron_obj` que los templates
         de crons ya consumen.

El guard de auth (require_auth) y la deteccion AJAX (is_ajax) vienen de
core.auth (antes estaban duplicados como _require_auth/_is_ajax).
"""
from __future__ import annotations

from django.http import HttpResponse

from core.auth import require_auth
from core.views import MaintainerViews

from . import services
from .forms import CronForm
from .models import Cron


class CronViews(MaintainerViews):
    """MaintainerViews especializado para crons (payload + toggle + contextos)."""

    # --- payload del service: los campos del form mapean 1:1 a create_cron /
    #     update_cron (name/slug/description/cron_expression/timezone/
    #     target_type/target_id/inputs/enabled).
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

    # --- list con filtros (tipo/habilitado). Guardamos el request para que
    #     do_list/list_context lean los GET; el resto lo arma el core.
    def list(self, request):
        self._list_request = request
        return super().list(request)

    # --- list: el listado de crons excluye los soft-deleted (el do_list
    #     generico del core no filtra deleted_at). Se delega al service de
    #     dominio, que ya devuelve la lista bajo la key `crons`.
    def do_list(self, search: str, page: int) -> dict:
        req = getattr(self, "_list_request", None)
        target_types = req.GET.getlist("target_type") if req else []
        # enabled: "" = sin filtro, "1" = True, "0" = False.
        val = req.GET.get("enabled") if req else None
        enabled = None if not val else (val == "1")
        return services.list_crons(
            search=search, page=page, per_page=self.per_page,
            target_types=target_types, enabled=enabled,
        )

    # --- toggle: el cron se habilita/deshabilita por el flag `enabled`, no por
    #     `status`. Se delega al service de dominio.
    def do_toggle(self, instance) -> str:
        enabled = services.toggle_cron_enabled(instance)
        return "habilitado" if enabled else "deshabilitado"

    # --- delete: soft delete propio (deleted_at + enabled=False).
    def do_delete(self, instance) -> None:
        services.delete_cron(instance)

    # --- contextos: los templates de crons usan `cron_obj` (no `object`).
    def list_context(self, data: dict, search: str) -> dict:
        ctx = super().list_context(data, search)
        ctx["page_title"] = "Crons"
        req = getattr(self, "_list_request", None)
        # Opciones + seleccion actual para el container de filtros.
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


# Instancia que cablea todo. list_key="crons" -> el template recibe la lista
# bajo `crons`. id_kwarg="cron_id" -> casa con <uuid:cron_id> de las URLs.
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
    """Export CSV (consolidado, abre en Excel) de los crons filtrados.
    Respeta los filtros activos: q (busqueda), target_type[] (multi) y
    enabled ("" sin filtro / "1" True / "0" False)."""
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
