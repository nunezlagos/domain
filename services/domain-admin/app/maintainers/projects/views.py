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




    def _form_payload(self, form) -> dict:


        return {
            "name": form.cleaned_data["name"],
            "slug": form.cleaned_data["slug"],
            "description": form.cleaned_data["description"],
            "repositories": self._parse_repo_post(form.data),
        }


    def _repo_rows(self, form, instance) -> list[dict]:

        if form.is_bound:
            return self._parse_repo_post(form.data)


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


    def list_context(self, data: dict, search: str) -> dict:
        ctx = super().list_context(data, search)
        ctx["page_title"] = "Proyectos"
        ctx["stats"] = services.get_stats()
        req = getattr(self, "_list_request", None)

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

        if mode == "edit" and instance is not None:

            ctx.update(_skills_ctx(instance, readonly=False))
            ctx.update(_rules_ctx(instance, readonly=False))
        return ctx

    def detail_context(self, instance) -> dict:
        ctx = {
            "project_obj": instance,
            "object": instance,
            "repositories": services.get_project_repositories(instance),
        }

        ctx.update(_skills_ctx(instance, readonly=True))
        ctx.update(_rules_ctx(instance, readonly=True))
        return ctx


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


def _skills_ctx(project, scope="all", page=1, readonly=False) -> dict:
    data = services.list_project_skills(project, scope=scope, page=page)
    return {
        "project_obj": project,
        "skills": data["items"],
        "skills_scope": data["scope"],
        "skills_applied_count": data["applied_count"],
        "skills_excluded_count": data["excluded_count"],
        "skills_global_count": data["global_count"],
        "skills_internal_count": data["internal_count"],
        "readonly": readonly,

        "total": data["total"], "page": data["page"], "per_page": data["per_page"],
        "total_pages": data["total_pages"], "has_prev": data["has_prev"], "has_next": data["has_next"],
    }


def _rules_ctx(project, scope="all", page=1, readonly=False) -> dict:
    data = services.list_project_rules(project, scope=scope, page=page)
    return {
        "project_obj": project,
        "rules": data["items"],
        "rules_scope": data["scope"],
        "rules_platform_count": data["platform_count"],
        "rules_project_count": data["project_count"],
        "readonly": readonly,
        "total": data["total"], "page": data["page"], "per_page": data["per_page"],
        "total_pages": data["total_pages"], "has_prev": data["has_prev"], "has_next": data["has_next"],
    }


def _resolve_project_or_redirect(project_id):
    try:
        return services.get_project(project_id), None
    except services.ProjectError as exc:
        return None, HttpResponseRedirect(views.url("list"))


@require_http_methods(["GET"])
def skills_pane(request, project_id):
    if (redir := require_auth(request)):
        return redir
    project, redir = _resolve_project_or_redirect(project_id)
    if redir:
        return redir
    scope = request.GET.get("scope") or "all"
    page = request.GET.get("page") or 1
    readonly = request.GET.get("readonly") == "1"
    return render(request, "projects/_skills_pane.html", _skills_ctx(project, scope, page, readonly))


@require_http_methods(["POST"])
def toggle_skill(request, project_id):
    if (redir := require_auth(request)):
        return redir
    project, redir = _resolve_project_or_redirect(project_id)
    if redir:
        return redir
    skill_id = request.POST.get("skill_id", "")
    op = request.POST.get("op", "")
    if skill_id and op in ("exclude", "include"):
        services.set_skill_excluded(project, skill_id, excluded=(op == "exclude"))
    scope = request.POST.get("scope") or "all"
    page = request.POST.get("page") or 1
    return render(request, "projects/_skills_pane.html", _skills_ctx(project, scope, page))


@require_http_methods(["GET"])
def rules_pane(request, project_id):
    if (redir := require_auth(request)):
        return redir
    project, redir = _resolve_project_or_redirect(project_id)
    if redir:
        return redir
    scope = request.GET.get("scope") or "all"
    page = request.GET.get("page") or 1
    readonly = request.GET.get("readonly") == "1"
    return render(request, "projects/_rules_pane.html", _rules_ctx(project, scope, page, readonly))


@require_http_methods(["POST"])
def toggle_rule(request, project_id):
    if (redir := require_auth(request)):
        return redir
    project, redir = _resolve_project_or_redirect(project_id)
    if redir:
        return redir
    policy_id = request.POST.get("policy_id", "")
    if policy_id:
        services.toggle_project_policy(project, policy_id)
    scope = request.POST.get("scope") or "all"
    page = request.POST.get("page") or 1
    return render(request, "projects/_rules_pane.html", _rules_ctx(project, scope, page))




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
