"""views (controllers) del mantenedor de Prompts.

================================ SDD ================================
HU: Como administrador de la organización quiero un mantenedor de
    Prompts para crear, listar, editar, activar/desactivar y dar de baja
    (soft) los prompts versionados que se usan en los agentes y flujos.

Criterios de aceptación:
  1. El listado muestra los prompts NO eliminados, paginado, con búsqueda
     server-side por slug/descripción/contenido (body).
  2. El listado se auto-refresca solo cuando la BD cambia (señal
     count + max(updated_at)), no con polling ciego.
  3. Crear/editar se hacen en un modal AJAX (?partial=1 + header
     X-Requested-With: fetch). Form inválido re-renderiza el modal con
     errores; submit OK recarga el listado.
  4. El detalle se abre en modal (?partial=1).
  5. Toggle alterna is_active true <-> false (POST). Eliminar es soft-delete
     (POST): marca deleted_at + is_active=False, NO borra la fila.
  6. La unicidad es (project_id, slug, version).
  7. Toda acción exige sesión autenticada; si no, redirige a /login/.
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
from .forms import PromptForm, PromptSearchForm


def _require_auth(request):
    """Redirect a /login/ si no está autenticado."""
    if not request.session.get("authenticated"):
        return HttpResponseRedirect("/login/")
    return None


def _is_ajax(request) -> bool:
    """El front manda el header literal 'fetch' (no 'XMLHttpRequest')."""
    return request.headers.get("X-Requested-With") == "fetch"


# === Listado ===

def prompt_list(request):
    if (redir := _require_auth(request)):
        return redir

    search_form = PromptSearchForm(request.GET or None)
    search = ""
    if search_form.is_valid():
        search = search_form.cleaned_data.get("q", "").strip()

    page_num = int(request.GET.get("page", 1) or 1)
    per_page = 20

    data = services.list_prompts(search=search, page=page_num, per_page=per_page)

    # ?fragment=table → solo tabla + paginación (sin base/layout), para
    # el refresh on-change y la paginación/búsqueda AJAX.
    if request.GET.get("fragment") == "table":
        return render(request, "prompts/_table_partial.html", {
            "prompts": data["prompts"],
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
    return render(request, "prompts/list.html", {
        "prompts": data["prompts"],
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


def prompt_list_signal(request):
    """Señal de cambios (JSON) para refresh on-change. Query barata."""
    if (redir := _require_auth(request)):
        return redir
    return JsonResponse(services.get_list_signal())


# === Detalle ===

def prompt_detail(request, prompt_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        prompt = services.get_prompt(prompt_id)
    except services.PromptError as exc:
        messages.error(request, str(exc))
        return HttpResponseRedirect(reverse("prompts:list"))

    ctx = {"prompt_obj": prompt}
    # ?partial=1 → solo el bloque detail (para modal).
    if request.GET.get("partial") == "1":
        return render(request, "prompts/_detail_partial.html", ctx)
    return render(request, "prompts/detail.html", ctx)


# === Crear ===

@require_http_methods(["GET", "POST"])
def prompt_create(request):
    if (redir := _require_auth(request)):
        return redir

    if request.method == "POST":
        form = PromptForm(request.POST)
        if form.is_valid():
            try:
                prompt = services.create_prompt(
                    project_id=form.cleaned_data.get("project_id"),
                    slug=form.cleaned_data["slug"],
                    version=form.cleaned_data["version"],
                    body=form.cleaned_data["body"],
                    description=form.cleaned_data["description"],
                    is_active=form.cleaned_data["is_active"],
                    tags=form.cleaned_data["tags"],
                )
                messages.success(
                    request, f"Prompt {prompt.display_name} creado correctamente."
                )
                if _is_ajax(request):
                    return HttpResponseRedirect(reverse("prompts:list"))
                return HttpResponseRedirect(reverse("prompts:detail", args=[prompt.pk]))
            except services.PromptError as exc:
                messages.error(request, str(exc))
                if _is_ajax(request):
                    return render(request, "prompts/_form_partial.html", {
                        "form": form, "mode": "create", "prompt_obj": None,
                        "action": reverse("prompts:create"),
                    })
    else:
        form = PromptForm()

    ctx = {"form": form, "mode": "create", "prompt_obj": None,
           "action": reverse("prompts:create")}
    if request.GET.get("partial") == "1":
        return render(request, "prompts/_form_partial.html", ctx)
    return render(request, "prompts/form.html", ctx)


# === Editar ===

@require_http_methods(["GET", "POST"])
def prompt_edit(request, prompt_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        prompt = services.get_prompt(prompt_id)
    except services.PromptError as exc:
        messages.error(request, str(exc))
        return HttpResponseRedirect(reverse("prompts:list"))

    if request.method == "POST":
        form = PromptForm(request.POST, instance=prompt)
        if form.is_valid():
            try:
                prompt = services.update_prompt(
                    prompt,
                    slug=form.cleaned_data["slug"],
                    version=form.cleaned_data["version"],
                    body=form.cleaned_data["body"],
                    description=form.cleaned_data["description"],
                    is_active=form.cleaned_data["is_active"],
                    tags=form.cleaned_data["tags"],
                )
                messages.success(request, f"Prompt {prompt.display_name} actualizado.")
                if _is_ajax(request):
                    return HttpResponseRedirect(reverse("prompts:list"))
                return HttpResponseRedirect(reverse("prompts:detail", args=[prompt.pk]))
            except services.PromptError as exc:
                messages.error(request, str(exc))
                if _is_ajax(request):
                    return render(request, "prompts/_form_partial.html", {
                        "form": form, "mode": "edit", "prompt_obj": prompt,
                        "action": reverse("prompts:edit", args=[prompt.pk]),
                    })
    else:
        form = PromptForm(instance=prompt)

    ctx = {"form": form, "mode": "edit", "prompt_obj": prompt,
           "action": reverse("prompts:edit", args=[prompt.pk])}
    if request.GET.get("partial") == "1":
        return render(request, "prompts/_form_partial.html", ctx)
    return render(request, "prompts/form.html", ctx)


# === Eliminar (soft) ===

@require_http_methods(["POST"])
def prompt_delete(request, prompt_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        prompt = services.get_prompt(prompt_id)
        services.delete_prompt(prompt)
        messages.success(
            request, f"Prompt {prompt.display_name} eliminado (soft delete)."
        )
    except services.PromptError as exc:
        messages.error(request, str(exc))

    return HttpResponseRedirect(reverse("prompts:list"))


# === Toggle activo/inactivo ===

@require_http_methods(["POST"])
def prompt_toggle(request, prompt_id: str):
    """Toggle de is_active true <-> false vía AJAX."""
    if (redir := _require_auth(request)):
        return redir

    try:
        prompt = services.get_prompt(prompt_id)
        new_active = services.toggle_prompt_status(prompt)
        label = "activado" if new_active else "desactivado"
        messages.success(request, f"Prompt {prompt.display_name} {label}.")
    except services.PromptError as exc:
        messages.error(request, str(exc))

    return HttpResponseRedirect(reverse("prompts:list"))
