"""Views del mantenedor de Prompts (migradas a core).

Las 7 vistas estándar (list, signal, detail, create, edit, delete, toggle) las
arma core.views.MaintainerViews. Acá solo:

  1. Se configura la instancia `views` (model/form/service/templates/labels).
  2. Se sobreescriben los hooks específicos de prompts:
       - do_create / do_update: el service espera kwargs concretos (slug, body,
         version, description, is_active, tags; project_id/created_by solo en
         alta). En edición project_id NO se manda (no se edita).
       - form_context / detail_context: exponen `prompt_obj` (no `object`), que
         los templates de prompts ya consumen.

El guard de auth (require_auth) y la detección AJAX (is_ajax) vienen de
core.auth (antes estaban duplicados como _require_auth/_is_ajax).
"""
from __future__ import annotations

from core.views import MaintainerViews

from . import services
from .forms import PromptForm
from .models import Prompt


class PromptViews(MaintainerViews):
    """MaintainerViews especializado para prompts (payload + context keys)."""

    # --- payload del service: project_id/created_by solo en alta; en edición
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

    # --- contextos: los templates de prompts usan `prompt_obj` (no `object`).
    def list_context(self, data: dict, search: str) -> dict:
        ctx = super().list_context(data, search)
        ctx["page_title"] = "Prompts"
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
