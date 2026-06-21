"""Views del mantenedor de Crons (schedules), migradas a core.

Las 7 vistas estándar (list, signal, detail, create, edit, delete, toggle) las
arma core.views.MaintainerViews. Acá solo:

  1. Se configura la instancia `views` (model/form/service/templates/labels).
  2. Se sobreescriben los hooks específicos de crons:
       - _form_payload: pasa los campos del form tal cual al service.
       - do_toggle: la dimensión alternable es el flag booleano `enabled`,
         NO `status` (el toggle genérico del core alterna status).
       - do_delete: soft delete propio (deleted_at + enabled=False).
       - form_context / detail_context: exponen `cron_obj` que los templates
         de crons ya consumen.

El guard de auth (require_auth) y la detección AJAX (is_ajax) vienen de
core.auth (antes estaban duplicados como _require_auth/_is_ajax).
"""
from __future__ import annotations

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

    # --- list: el listado de crons excluye los soft-deleted (el do_list
    #     genérico del core no filtra deleted_at). Se delega al service de
    #     dominio, que ya devuelve la lista bajo la key `crons`.
    def do_list(self, search: str, page: int) -> dict:
        return services.list_crons(search=search, page=page, per_page=self.per_page)

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
    per_page=20,
    search_param="q",
)
