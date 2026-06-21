"""views (controllers) del mantenedor de Skills.

================================ SDD ================================
HU: Como administrador de la organización quiero un mantenedor de Skills
    para crear, listar, editar y dar de baja (soft) las skills de la
    plataforma, y consultar sus versiones (snapshots) en modo lectura.

Criterios de aceptación:
  1. El listado muestra las skills NO eliminadas, paginado, con búsqueda
     server-side por nombre/slug/descripción.
  2. El listado se auto-refresca solo cuando la BD cambia (señal
     count + max(updated_at)), no con polling ciego.
  3. Crear/editar se hacen en un modal AJAX (?partial=1 + header
     X-Requested-With: fetch). Form inválido re-renderiza el modal con
     errores; submit OK recarga el listado.
  4. El detalle se abre en modal (?partial=1) e incluye la lista READ-ONLY
     de versiones (skill_versions): número de versión, changelog, autor y
     fecha. No hay CRUD sobre versiones.
  5. Eliminar es soft-delete (POST): marca deleted_at, NO borra la fila.
     skills NO tiene columna `status`, por lo tanto NO hay toggle de estado.
  6. Toda acción exige sesión autenticada; si no, redirige a /login/.
====================================================================

Cada view:
1. Auth guard.
2. Llama al service correspondiente (lógica de negocio).
3. Renderiza template (o redirige). Errores de dominio -> messages.error.
"""
from __future__ import annotations

from django.contrib import messages
from django.http import HttpResponseRedirect, JsonResponse
from django.shortcuts import render
from django.urls import reverse
from django.views.decorators.http import require_http_methods

from . import services
from .forms import SkillForm, SkillSearchForm


def _require_auth(request):
    """Redirect a /login/ si no está autenticado."""
    if not request.session.get("authenticated"):
        return HttpResponseRedirect("/login/")
    return None


def _is_ajax(request) -> bool:
    """El front manda el header literal 'fetch' (no 'XMLHttpRequest')."""
    return request.headers.get("X-Requested-With") == "fetch"


# === Listado ===

def skill_list(request):
    if (redir := _require_auth(request)):
        return redir

    search_form = SkillSearchForm(request.GET or None)
    search = ""
    if search_form.is_valid():
        search = search_form.cleaned_data.get("q", "").strip()

    page_num = int(request.GET.get("page", 1) or 1)
    per_page = 20

    data = services.list_skills(search=search, page=page_num, per_page=per_page)

    # ?fragment=table → solo tabla + paginación (sin base/layout), para el
    # refresh on-change y la paginación/búsqueda AJAX.
    if request.GET.get("fragment") == "table":
        return render(request, "skills/_table_partial.html", {
            "skills": data["skills"],
            "total": data["total"],
            "page": data["page"],
            "per_page": data["per_page"],
            "total_pages": data["total_pages"],
            "has_next": data["has_next"],
            "has_prev": data["has_prev"],
            "search": search,
        })

    # Señal inicial embebida: el front parte con la versión exacta del render.
    sig = services.get_list_signal()
    return render(request, "skills/list.html", {
        "skills": data["skills"],
        "total": data["total"],
        "page": data["page"],
        "per_page": data["per_page"],
        "total_pages": data["total_pages"],
        "has_next": data["has_next"],
        "has_prev": data["has_prev"],
        "search": search,
        "search_form": search_form,
        "signal_count": sig["count"],
        "signal_version": sig["version"],
    })


def skill_list_signal(request):
    """Señal de cambios (JSON) para refresh on-change. Query barata."""
    if (redir := _require_auth(request)):
        return redir
    return JsonResponse(services.get_list_signal())


# === Detalle ===

def skill_detail(request, skill_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        skill = services.get_skill(skill_id)
    except services.SkillError as exc:
        messages.error(request, str(exc))
        return HttpResponseRedirect(reverse("skills:list"))

    ctx = {
        "skill_obj": skill,
        "skill_versions": services.get_skill_versions(skill),
    }
    # ?partial=1 → solo el bloque detail (para modal).
    if request.GET.get("partial") == "1":
        return render(request, "skills/_detail_partial.html", ctx)
    return render(request, "skills/detail.html", ctx)


# === Crear ===

@require_http_methods(["GET", "POST"])
def skill_create(request):
    if (redir := _require_auth(request)):
        return redir

    if request.method == "POST":
        form = SkillForm(request.POST)
        if form.is_valid():
            try:
                skill = services.create_skill(
                    slug=form.cleaned_data["slug"],
                    name=form.cleaned_data["name"],
                    skill_type=form.cleaned_data["skill_type"],
                    description=form.cleaned_data["description"],
                    content=form.cleaned_data["content"],
                    timeout_seconds=form.cleaned_data["timeout_seconds"],
                    idempotent=form.cleaned_data["idempotent"],
                    has_side_effects=form.cleaned_data["has_side_effects"],
                    tags=form.cleaned_data["tags"],
                )
                messages.success(request, f"Skill {skill.name} creada correctamente.")
                if _is_ajax(request):
                    return HttpResponseRedirect(reverse("skills:list"))
                return HttpResponseRedirect(reverse("skills:detail", args=[skill.pk]))
            except services.SkillError as exc:
                messages.error(request, str(exc))
                if _is_ajax(request):
                    return render(request, "skills/_form_partial.html", {
                        "form": form, "mode": "create", "skill_obj": None,
                        "action": reverse("skills:create"),
                    })
    else:
        form = SkillForm()

    ctx = {"form": form, "mode": "create", "skill_obj": None,
           "action": reverse("skills:create")}
    if request.GET.get("partial") == "1":
        return render(request, "skills/_form_partial.html", ctx)
    return render(request, "skills/form.html", ctx)


# === Editar ===

@require_http_methods(["GET", "POST"])
def skill_edit(request, skill_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        skill = services.get_skill(skill_id)
    except services.SkillError as exc:
        messages.error(request, str(exc))
        return HttpResponseRedirect(reverse("skills:list"))

    if request.method == "POST":
        form = SkillForm(request.POST, instance=skill)
        if form.is_valid():
            try:
                skill = services.update_skill(
                    skill,
                    slug=form.cleaned_data["slug"],
                    name=form.cleaned_data["name"],
                    skill_type=form.cleaned_data["skill_type"],
                    description=form.cleaned_data["description"],
                    content=form.cleaned_data["content"],
                    timeout_seconds=form.cleaned_data["timeout_seconds"],
                    idempotent=form.cleaned_data["idempotent"],
                    has_side_effects=form.cleaned_data["has_side_effects"],
                    tags=form.cleaned_data["tags"],
                )
                messages.success(request, f"Skill {skill.name} actualizada.")
                if _is_ajax(request):
                    return HttpResponseRedirect(reverse("skills:list"))
                return HttpResponseRedirect(reverse("skills:detail", args=[skill.pk]))
            except services.SkillError as exc:
                messages.error(request, str(exc))
                if _is_ajax(request):
                    return render(request, "skills/_form_partial.html", {
                        "form": form, "mode": "edit", "skill_obj": skill,
                        "action": reverse("skills:edit", args=[skill.pk]),
                    })
    else:
        form = SkillForm(instance=skill)

    ctx = {"form": form, "mode": "edit", "skill_obj": skill,
           "action": reverse("skills:edit", args=[skill.pk])}
    if request.GET.get("partial") == "1":
        return render(request, "skills/_form_partial.html", ctx)
    return render(request, "skills/form.html", ctx)


# === Eliminar (soft) ===

@require_http_methods(["POST"])
def skill_delete(request, skill_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        skill = services.get_skill(skill_id)
        services.delete_skill(skill)
        messages.success(request, f"Skill {skill.name} eliminada (soft delete).")
    except services.SkillError as exc:
        messages.error(request, str(exc))

    return HttpResponseRedirect(reverse("skills:list"))
