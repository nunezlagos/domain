"""Mantenedor de Proyectos — views (controllers en MVC).

================================================================
SDD — Spec-Driven Design
----------------------------------------------------------------
HU: Como administrador de la plataforma quiero gestionar los Proyectos
    de la organización (alta, edición, archivo/restauración y baja) desde
    el panel admin, replicando el patrón del mantenedor de Usuarios, para
    administrar la entidad `projects` (con sus templates y remotos git)
    sin tocar la base de datos directamente.

Criterios de aceptación:
  CA1. El listado muestra solo proyectos activos (deleted_at IS NULL),
       con búsqueda server-side (nombre/slug/descripción/repo) y paginación.
  CA2. El listado se auto-refresca SOLO al detectar cambios en la BD
       (señal count + max(updated_at) vía endpoint `signal`), no por polling
       ciego — incluye cambios hechos por domain-mcp u otros admins.
  CA3. Crear/editar funciona como página standalone (fallback no-JS) y como
       modal AJAX (?partial=1 devuelve el form parcial; submit con header
       X-Requested-With: fetch redirige al listado para que el JS recargue).
  CA4. El detalle funciona standalone y como modal (?partial=1), y lista los
       remotos git (project_repositories) del proyecto.
  CA5. El estado activo/archivado vive en la columna status y en deleted_at
       (soft-delete), mantenidos en sync. "Eliminar" archiva (soft-delete) y
       "toggle" archiva/restaura.
  CA6. Toda view exige sesión autenticada; sin ella redirige a /login/.
================================================================

Cada view:
1. Recibe request HTTP
2. Llama al service correspondiente (lógica de negocio)
3. Renderiza template (o redirige)

Mensajes de feedback vía Django messages framework.
"""
from __future__ import annotations

from django.contrib import messages
from django.http import HttpResponseRedirect, JsonResponse
from django.shortcuts import render
from django.urls import reverse
from django.views.decorators.http import require_http_methods

from . import services
from .forms import ProjectForm, ProjectSearchForm


def _require_auth(request):
    """Redirect a /login/ si no está autenticado."""
    if not request.session.get("authenticated"):
        return HttpResponseRedirect("/login/")
    return None


def _is_ajax(request) -> bool:
    """El front manda el header literal 'fetch' (NO 'XMLHttpRequest')."""
    return request.headers.get("X-Requested-With") == "fetch"


# === Listado ===

def project_list(request):
    if (redir := _require_auth(request)):
        return redir

    search_form = ProjectSearchForm(request.GET or None)
    search = ""
    if search_form.is_valid():
        search = search_form.cleaned_data.get("q", "").strip()

    page_num = int(request.GET.get("page", 1) or 1)
    per_page = 20

    data = services.list_projects(search=search, page=page_num, per_page=per_page)

    # ?fragment=table → solo tabla + paginación (sin base/layout), para refresh.
    if request.GET.get("fragment") == "table":
        return render(request, "projects/_table_partial.html", {
            "projects": data["projects"],
            "total": data["total"],
            "page": data["page"],
            "per_page": data["per_page"],
            "total_pages": data["total_pages"],
            "has_next": data["has_next"],
            "has_prev": data["has_prev"],
            "search": search,
        })

    # Señal inicial embebida: el front parte con la versión exacta del render
    # (evita perder un cambio entre el render y el primer poll).
    sig = services.get_list_signal()
    return render(request, "projects/list.html", {
        "projects": data["projects"],
        "total": data["total"],
        "page": data["page"],
        "per_page": data["per_page"],
        "total_pages": data["total_pages"],
        "has_next": data["has_next"],
        "has_prev": data["has_prev"],
        "search": search,
        "search_form": search_form,
        "stats": services.get_stats(),
        "signal_count": sig["count"],
        "signal_version": sig["version"],
    })


def project_list_signal(request):
    """Señal de cambios (JSON) para refresh on-change.

    Reemplaza el polling ciego de la tabla: el front pega acá (query barata),
    compara con su última señal y solo refresca la tabla si cambió.
    """
    if (redir := _require_auth(request)):
        return redir
    return JsonResponse(services.get_list_signal())


# === Detalle ===

def project_detail(request, project_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        project = services.get_project(project_id)
    except services.ProjectError as exc:
        messages.error(request, str(exc))
        return HttpResponseRedirect(reverse("projects:list"))

    repositories = services.get_project_repositories(project)

    ctx = {
        "project_obj": project,
        "repositories": repositories,
    }
    # ?partial=1 → solo el bloque detail (para modal)
    if request.GET.get("partial") == "1":
        return render(request, "projects/_detail_partial.html", ctx)
    return render(request, "projects/detail.html", ctx)


# === Crear ===

@require_http_methods(["GET", "POST"])
def project_create(request):
    if (redir := _require_auth(request)):
        return redir

    if request.method == "POST":
        form = ProjectForm(request.POST)
        if form.is_valid():
            try:
                project = services.create_project(
                    name=form.cleaned_data["name"],
                    slug=form.cleaned_data["slug"],
                    description=form.cleaned_data["description"],
                    repository_url=form.cleaned_data["repository_url"],
                    template_id=form.cleaned_data["template"],
                    current_branch=form.cleaned_data["current_branch"],
                )
                messages.success(request, f"Proyecto {project.name} creado correctamente.")
                if _is_ajax(request):
                    return HttpResponseRedirect(reverse("projects:list"))
                return HttpResponseRedirect(reverse("projects:detail", args=[project.pk]))
            except services.ProjectError as exc:
                messages.error(request, str(exc))
                form.add_error(None, str(exc))
                if _is_ajax(request):
                    return render(request, "projects/_form_partial.html", {
                        "form": form, "mode": "create", "project_obj": None,
                        "action": reverse("projects:create"),
                    })
    else:
        form = ProjectForm()

    ctx = {"form": form, "mode": "create", "project_obj": None,
           "action": reverse("projects:create")}
    if request.GET.get("partial") == "1":
        return render(request, "projects/_form_partial.html", ctx)
    return render(request, "projects/form.html", ctx)


# === Editar ===

@require_http_methods(["GET", "POST"])
def project_edit(request, project_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        project = services.get_project(project_id)
    except services.ProjectError as exc:
        messages.error(request, str(exc))
        return HttpResponseRedirect(reverse("projects:list"))

    if request.method == "POST":
        form = ProjectForm(request.POST, instance=project)
        if form.is_valid():
            try:
                project = services.update_project(
                    project,
                    name=form.cleaned_data["name"],
                    slug=form.cleaned_data["slug"],
                    description=form.cleaned_data["description"],
                    repository_url=form.cleaned_data["repository_url"],
                    template_id=form.cleaned_data["template"],
                    current_branch=form.cleaned_data["current_branch"],
                )
                messages.success(request, f"Proyecto {project.name} actualizado.")
                if _is_ajax(request):
                    return HttpResponseRedirect(reverse("projects:list"))
                return HttpResponseRedirect(reverse("projects:detail", args=[project.pk]))
            except services.ProjectError as exc:
                messages.error(request, str(exc))
                form.add_error(None, str(exc))
                if _is_ajax(request):
                    return render(request, "projects/_form_partial.html", {
                        "form": form, "mode": "edit", "project_obj": project,
                        "action": reverse("projects:edit", args=[project.pk]),
                    })
    else:
        form = ProjectForm(instance=project)

    ctx = {"form": form, "mode": "edit", "project_obj": project,
           "action": reverse("projects:edit", args=[project.pk])}
    if request.GET.get("partial") == "1":
        return render(request, "projects/_form_partial.html", ctx)
    return render(request, "projects/form.html", ctx)


# === Eliminar (soft delete = archivar) ===

@require_http_methods(["POST"])
def project_delete(request, project_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        project = services.get_project(project_id)
        services.delete_project(project)
        messages.success(request, f"Proyecto {project.name} eliminado (soft delete).")
    except services.ProjectError as exc:
        messages.error(request, str(exc))

    return HttpResponseRedirect(reverse("projects:list"))


# === Toggle activo/archivado (vía deleted_at) ===

@require_http_methods(["POST"])
def project_toggle(request, project_id: str):
    """Archiva o restaura el proyecto (toggle de deleted_at) vía AJAX."""
    if (redir := _require_auth(request)):
        return redir

    try:
        project = services.get_project(project_id)
        new_status = services.toggle_project_status(project)
        label = "archivado" if new_status == project.STATUS_ARCHIVED else "restaurado"
        messages.success(request, f"Proyecto {project.name} {label}.")
    except services.ProjectError as exc:
        messages.error(request, str(exc))

    # AJAX: redirige al list; el JS hace reload para mostrar el cambio.
    return HttpResponseRedirect(reverse("projects:list"))
