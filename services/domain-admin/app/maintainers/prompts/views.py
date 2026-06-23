"""Views del mantenedor de Prompts (migradas a core).

Las 7 vistas estandar (list, signal, detail, create, edit, delete, toggle) las
arma core.views.MaintainerViews. Aqui solo:

  1. Se configura la instancia `views` (model/form/service/templates/labels).
  2. Se sobreescriben los hooks especificos de prompts:
       - do_create / do_update: el service espera kwargs concretos (slug, body,
         version, description, is_active, tags; project_id/created_by solo en
         alta). En edicion project_id NO se manda (no se edita).
       - form_context / detail_context: exponen `prompt_obj` (no `object`), que
         los templates de prompts ya consumen.

El guard de auth (require_auth) y la deteccion AJAX (is_ajax) vienen de
core.auth (antes estaban duplicados como _require_auth/_is_ajax).
"""
from __future__ import annotations

from django.http import HttpResponse

from core.auth import require_auth
from core.views import MaintainerViews

from . import services
from .forms import PromptForm
from .models import Prompt


class PromptViews(MaintainerViews):
    """MaintainerViews especializado para prompts (payload + context keys)."""

    # --- payload del service: project_id/created_by solo en alta; en edicion
    #     el service no acepta project_id (no se edita una vez creado).
    def do_create(self, form):
        cd = form.cleaned_data
        return services.create_prompt(
            project_id=cd.get("project_id"),
            slug=cd["slug"],
            version=cd["version"],
            body=cd["body"],
            description=cd["description"],
            is_active=cd["is_active"],
            tags=cd["tags"],
        )

    def do_update(self, instance, form):
        cd = form.cleaned_data
        return services.update_prompt(
            instance,
            slug=cd["slug"],
            version=cd["version"],
            body=cd["body"],
            description=cd["description"],
            is_active=cd["is_active"],
            tags=cd["tags"],
        )

    # --- list con filtro de estado (is_active). Guardamos el request para que
    #     do_list/list_context lean el GET; el resto lo arma core.
    def list(self, request):
        self._list_request = request
        return super().list(request)

    def do_list(self, search: str, page: int) -> dict:
        req = getattr(self, "_list_request", None)
        val = req.GET.get("active") if req else ""
        is_active = None if not val else (val == "1")
        return services.list_prompts(
            search=search, page=page, per_page=self.per_page,
            is_active=is_active,
        )

    # --- contextos: los templates de prompts usan `prompt_obj` (no `object`).
    def list_context(self, data: dict, search: str) -> dict:
        ctx = super().list_context(data, search)
        ctx["page_title"] = "Prompts"
        req = getattr(self, "_list_request", None)
        # Seleccion actual del filtro de estado para el container de filtros.
        ctx["selected_active"] = req.GET.get("active") if req else ""
        return ctx

    def form_context(self, form, mode: str, instance, action: str) -> dict:
        return {
            "form": form,
            "mode": mode,
            "prompt_obj": instance,
            "object": instance,
            "action": action,
        }

    def detail_context(self, instance) -> dict:
        return {"prompt_obj": instance, "object": instance}


# Instancia que cablea todo. list_key="prompts" -> el template recibe la lista
# bajo `prompts`. id_kwarg="prompt_id" -> casa con <uuid:prompt_id> de las URLs.
views = PromptViews(
    app_name="prompts",
    model=Prompt,
    form_class=PromptForm,
    service=services,
    templates="prompts",
    search_fields=("slug", "description", "body"),
    entity_label="Prompt",
    id_kwarg="prompt_id",
    list_key="prompts",
    per_page=20,
    search_param="q",
)


def export_prompts(request):
    """Export CSV (consolidado, abre en Excel) de los prompts filtrados.
    Respeta los filtros activos: q (busqueda) y active (is_active)."""
    if (redir := require_auth(request)):
        return redir
    val = request.GET.get("active") or ""
    is_active = None if not val else (val == "1")
    csv_data = services.export_prompts_csv(
        search=(request.GET.get("q") or "").strip(),
        is_active=is_active,
    )
    resp = HttpResponse(csv_data, content_type="text/csv; charset=utf-8")
    resp["Content-Disposition"] = 'attachment; filename="prompts.csv"'
    return resp
