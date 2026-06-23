"""Views del mantenedor de Proyectos (migradas a core).

Las 7 vistas estandar (list, signal, detail, create, edit, delete, toggle) las
arma core.views.MaintainerViews. Aqui solo:

  1. Se configura la instancia `views` (model/form/service/templates/labels).
  2. Se sobreescriben los hooks especificos de projects:
       - _form_payload: mapea el campo `template` del form -> `template_id` que
         espera el service (y omite los campos que el service no recibe).
       - list_context: agrega stats + page_title que el listado consume.
       - form_context / detail_context: exponen `project_obj` (+ repositories en
         detail) que los templates de projects ya consumen.
       - toggle: mensaje propio (archivado/restaurado) en vez del generico.

El guard de auth (require_auth) y la deteccion AJAX (is_ajax) vienen de
core.auth (antes estaban duplicados como _require_auth/_is_ajax).
"""
from __future__ import annotations

from django.contrib import messages
from django.http import HttpResponse, HttpResponseRedirect
from django.shortcuts import render
from django.views.decorators.http import require_http_methods

from core.auth import require_auth
from core.views import MaintainerViews

from . import services
from .forms import ProjectForm
from .models import Project


class ProjectViews(MaintainerViews):
    """MaintainerViews especializado para projects (context keys + payload + msgs)."""

    # --- repos: filas dinamicas (url + rama + folder) que viajan como arrays
    #     paralelos repo_url[]/repo_branch[]/repo_folder[] en el POST.
    @staticmethod
    def _parse_repo_post(data) -> list[dict]:
        urls = data.getlist("repo_url")
        branches = data.getlist("repo_branch")
        folders = data.getlist("repo_folder")
        rows: list[dict] = []
        for i, url in enumerate(urls):
            url = (url or "").strip()
            if not url:
                continue
            rows.append({
                "url": url,
                "branch_default": (branches[i] if i < len(branches) else "").strip(),
                "root_path": (folders[i] if i < len(folders) else "").strip(),
            })
        return rows

    # --- payload del service: campos base + el set completo de repos parseado
    #     del POST. La URL principal y el template ya no se mandan (la primera
    #     se deriva del repo default; el template se quito).
    def _form_payload(self, form) -> dict:
        # current_branch ya no se edita en el modal (cada repo tiene su rama);
        # se omite para preservar el valor existente (es referencial / de sistema).
        return {
            "name": form.cleaned_data["name"],
            "slug": form.cleaned_data["slug"],
            "description": form.cleaned_data["description"],
            "repositories": self._parse_repo_post(form.data),
        }

    # --- filas de repos para el template del form (pre-fill).
    def _repo_rows(self, form, instance) -> list[dict]:
        # Re-render por error de validacion (form bound): preservar lo tipeado.
        if form.is_bound:
            return self._parse_repo_post(form.data)
        # GET edit: repos existentes; si no hay pero hay URL principal legacy,
        # sembrar una fila para no perder el dato.
        if instance is not None:
            repos = services.get_project_repositories(instance)
            if repos:
                return [
                    {"url": r.url, "branch_default": r.branch_default, "root_path": r.root_path}
                    for r in repos
                ]
            if instance.repository_url:
                return [{
                    "url": instance.repository_url,
                    "branch_default": instance.current_branch,
                    "root_path": "",
                }]
        return []

    # --- list con filtro por estado. Guardamos el request para que
    #     do_list/list_context lean el GET; el resto lo arma core.
    def list(self, request):
        self._list_request = request
        return super().list(request)

    def do_list(self, search: str, page: int) -> dict:
        req = getattr(self, "_list_request", None)
        status = req.GET.get("status") if req else ""
        return services.list_projects(
            search=search, page=page, per_page=self.per_page,
            statuses=[status] if status else None,
        )

    # --- contextos: los templates de projects usan `project_obj` (no `object`).
    def list_context(self, data: dict, search: str) -> dict:
        ctx = super().list_context(data, search)
        ctx["page_title"] = "Proyectos"
        ctx["stats"] = services.get_stats()
        req = getattr(self, "_list_request", None)
        # Opciones + seleccion actual para el container de filtros.
        ctx["status_options"] = Project.STATUS_CHOICES
        ctx["selected_status"] = req.GET.get("status") if req else ""
        return ctx

    def form_context(self, form, mode: str, instance, action: str) -> dict:
        ctx = {
            "form": form,
            "mode": mode,
            "project_obj": instance,
            "object": instance,
            "action": action,
            "repo_rows": self._repo_rows(form, instance),
        }
        # En edicion exponemos skills + reglas para las tabs (mismo set que el ver).
        if mode == "edit" and instance is not None:
            ctx.update(_skills_ctx(instance))
            ctx.update(_rules_ctx(instance))
        return ctx

    def detail_context(self, instance) -> dict:
        ctx = {
            "project_obj": instance,
            "object": instance,
            "repositories": services.get_project_repositories(instance),
        }
        ctx.update(_skills_ctx(instance))
        ctx.update(_rules_ctx(instance))
        return ctx

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


def _skills_ctx(project) -> dict:
    """Contexto del pane de skills (globales + internas, con flag .excluded)."""
    data = services.list_project_skills(project)
    return {
        "project_obj": project,
        "skills_globals": data["globals"],
        "skills_internals": data["internals"],
        "skills_applied_count": data["applied_count"],
        "skills_excluded_count": data["excluded_count"],
    }


def _rules_ctx(project) -> dict:
    """Contexto del pane de reglas (plataforma auto + del proyecto)."""
    return {
        "project_obj": project,
        "platform_policies": services.list_platform_policies(),
        "project_policies": services.list_project_policies(project),
    }


@require_http_methods(["POST"])
def toggle_skill(request, project_id):
    """Excluye (op=exclude) o re-incluye (op=include) una skill para el proyecto;
    re-renderiza SOLO el pane de skills (#project-skills-pane)."""
    if (redir := require_auth(request)):
        return redir
    try:
        project = services.get_project(project_id)
    except services.ProjectError as exc:
        messages.error(request, str(exc))
        return HttpResponseRedirect(views.url("list"))
    skill_id = request.POST.get("skill_id", "")
    op = request.POST.get("op", "")
    if skill_id and op in ("exclude", "include"):
        services.set_skill_excluded(project, skill_id, excluded=(op == "exclude"))
    return render(request, "projects/_skills_pane.html", _skills_ctx(project))


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
    per_page=10,
    search_param="q",
)


def export_projects(request):
    """Export CSV (consolidado, abre en Excel) de los proyectos filtrados.
    Respeta los filtros activos: q (busqueda) y status (estado)."""
    if (redir := require_auth(request)):
        return redir
    status = (request.GET.get("status") or "").strip()
    csv_data = services.export_projects_csv(
        search=(request.GET.get("q") or "").strip(),
        statuses=[status] if status else None,
    )
    resp = HttpResponse(csv_data, content_type="text/csv; charset=utf-8")
    resp["Content-Disposition"] = 'attachment; filename="proyectos.csv"'
    return resp
