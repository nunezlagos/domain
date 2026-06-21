"""Views del mantenedor de Proyectos (migradas a core).

Las 7 vistas estándar (list, signal, detail, create, edit, delete, toggle) las
arma core.views.MaintainerViews. Acá solo:

  1. Se configura la instancia `views` (model/form/service/templates/labels).
  2. Se sobreescriben los hooks específicos de projects:
       - _form_payload: mapea el campo `template` del form -> `template_id` que
         espera el service (y omite los campos que el service no recibe).
       - list_context: agrega stats + page_title que el listado consume.
       - form_context / detail_context: exponen `project_obj` (+ repositories en
         detail) que los templates de projects ya consumen.
       - toggle: mensaje propio (archivado/restaurado) en vez del genérico.

El guard de auth (require_auth) y la detección AJAX (is_ajax) vienen de
core.auth (antes estaban duplicados como _require_auth/_is_ajax).
"""
from __future__ import annotations

from django.contrib import messages
from django.http import HttpResponseRedirect

from core.views import MaintainerViews

from . import services
from .forms import ProjectForm
from .models import Project


class ProjectViews(MaintainerViews):
    """MaintainerViews especializado para projects (context keys + payload + msgs)."""

    # --- payload del service: el form trae `template`; el service espera
    #     `template_id`. Además omitimos campos que el service no recibe.
    def _form_payload(self, form) -> dict:
        return {
            "name": form.cleaned_data["name"],
            "slug": form.cleaned_data["slug"],
            "description": form.cleaned_data["description"],
            "repository_url": form.cleaned_data["repository_url"],
            "template_id": form.cleaned_data["template"],
            "current_branch": form.cleaned_data["current_branch"],
        }

    # --- contextos: los templates de projects usan `project_obj` (no `object`).
    def list_context(self, data: dict, search: str) -> dict:
        ctx = super().list_context(data, search)
        ctx["page_title"] = "Proyectos"
        ctx["stats"] = services.get_stats()
        return ctx

    def form_context(self, form, mode: str, instance, action: str) -> dict:
        return {
            "form": form,
            "mode": mode,
            "project_obj": instance,
            "object": instance,
            "action": action,
        }

    def detail_context(self, instance) -> dict:
        return {
            "project_obj": instance,
            "object": instance,
            "repositories": services.get_project_repositories(instance),
        }

    # --- toggle con feedback de dominio (archivado/restaurado).
    def toggle(self, request, **kwargs):
        from core.auth import require_auth

        if (redir := require_auth(request)):
            return redir
        obj_id = kwargs[self.id_kwarg]
        try:
            project = self.do_get(obj_id)
            new_status = self.do_toggle(project)
            label = "archivado" if new_status == Project.STATUS_ARCHIVED else "restaurado"
            messages.success(request, f"Proyecto {project.name} {label}.")
        except self.error_class as exc:
            messages.error(request, str(exc))
        return HttpResponseRedirect(self.url("list"))


# Instancia que cablea todo. list_key="projects" -> el template recibe la lista
# bajo `projects`. id_kwarg="project_id" -> casa con <uuid:project_id> de las URLs.
views = ProjectViews(
    app_name="projects",
    model=Project,
    form_class=ProjectForm,
    service=services,
    templates="projects",
    search_fields=("name", "slug", "description", "repository_url"),
    entity_label="Proyecto",
    id_kwarg="project_id",
    list_key="projects",
    per_page=20,
    search_param="q",
)
