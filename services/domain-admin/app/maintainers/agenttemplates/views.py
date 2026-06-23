"""Views del mantenedor de Plantillas de Agentes.

Las 7 vistas estandar (list, signal, detail, create, edit, delete, toggle) las
arma core.views.MaintainerViews. Aqui solo:

  1. Se configura la instancia `views` (model/form/service/templates/labels).
  2. Se sobreescriben los context builders: los templates usan `template_obj`
     (no `object`).

agent_templates NO tiene deleted_at → la baja es HARD delete (services.
delete_agenttemplate borra la fila). NO hay toggle de status en la UI: el boton
simplemente no se renderiza en los templates (la ruta queda cableada por el
helper igual que en el resto de los mantenedores, pero no se usa).

El guard de auth (require_auth) y la deteccion AJAX (is_ajax) vienen de
core.auth.
"""
from __future__ import annotations

from core.views import MaintainerViews

from . import services
from .forms import AgentTemplateForm
from .models import AgentTemplate


class AgentTemplateViews(MaintainerViews):
    """MaintainerViews especializado para plantillas de agentes."""

    # core.do_list usa el MaintainerService generico; delegamos en
    # services.list_agenttemplates para devolver la lista bajo `agenttemplates`
    # (list_key) con el orden por name del service.
    def do_list(self, search: str, page: int) -> dict:
        return services.list_agenttemplates(
            search=search, page=page, per_page=self.per_page
        )

    def list_context(self, data: dict, search: str) -> dict:
        ctx = super().list_context(data, search)
        ctx["page_title"] = "Plantillas de Agentes"
        return ctx

    def form_context(self, form, mode: str, instance, action: str) -> dict:
        return {
            "form": form,
            "mode": mode,
            "template_obj": instance,
            "object": instance,
            "action": action,
        }

    def detail_context(self, instance) -> dict:
        return {
            "template_obj": instance,
            "object": instance,
        }


# Instancia que cablea todo. list_key="agenttemplates" -> el template recibe la
# lista bajo `agenttemplates`. id_kwarg="template_id" -> casa con
# <uuid:template_id> de las URLs. entity_label="Plantilla de Agente" -> core
# descubre get_plantilla_de_agente/create_…/update_…/delete_… (alias del service).
views = AgentTemplateViews(
    app_name="agenttemplates",
    model=AgentTemplate,
    form_class=AgentTemplateForm,
    service=services,
    templates="agenttemplates",
    search_fields=("name", "slug", "role"),
    entity_label="Plantilla de Agente",
    id_kwarg="template_id",
    list_key="agenttemplates",
    per_page=10,
    search_param="q",
)
