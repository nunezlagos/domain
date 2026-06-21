"""views (controllers) del mantenedor de Crons (schedules).

================================ SDD ================================
HU: Como administrador de la organización quiero un mantenedor de
    Crons (schedules) para crear, listar, editar, habilitar/deshabilitar
    y dar de baja (soft) los schedules definidos por el usuario que
    disparan un target (flow/agent/skill) según una expresión cron.

Criterios de aceptación:
  1. El listado muestra los crons NO eliminados, paginado, con búsqueda
     server-side por nombre/slug/expresión/target_type.
  2. El listado se auto-refresca solo cuando la BD cambia (señal
     count + max(updated_at)), no con polling ciego.
  3. Crear/editar se hacen en un modal AJAX (?partial=1 + header
     X-Requested-With: fetch). Form inválido re-renderiza el modal con
     errores; submit OK recarga el listado.
  4. El detalle se abre en modal (?partial=1).
  5. Toggle alterna enabled True <-> False (POST). Eliminar es soft-delete
     (POST): marca deleted_at + enabled=False, NO borra la fila.
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
from .forms import CronForm, CronSearchForm


def _require_auth(request):
    """Redirect a /login/ si no está autenticado."""
    if not request.session.get("authenticated"):
        return HttpResponseRedirect("/login/")
    return None


def _is_ajax(request) -> bool:
    """El front manda el header literal 'fetch' (no 'XMLHttpRequest')."""
    return request.headers.get("X-Requested-With") == "fetch"


# === Listado ===

def cron_list(request):
    if (redir := _require_auth(request)):
        return redir

    search_form = CronSearchForm(request.GET or None)
    search = ""
    if search_form.is_valid():
        search = search_form.cleaned_data.get("q", "").strip()

    page_num = int(request.GET.get("page", 1) or 1)
    per_page = 20

    data = services.list_crons(search=search, page=page_num, per_page=per_page)

    # ?fragment=table → solo tabla + paginación (sin base/layout), para
    # el refresh on-change y la paginación/búsqueda AJAX.
    if request.GET.get("fragment") == "table":
        return render(request, "crons/_table_partial.html", {
            "crons": data["crons"],
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
    return render(request, "crons/list.html", {
        "crons": data["crons"],
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


def cron_list_signal(request):
    """Señal de cambios (JSON) para refresh on-change. Query barata."""
    if (redir := _require_auth(request)):
        return redir
    return JsonResponse(services.get_list_signal())


# === Detalle ===

def cron_detail(request, cron_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        cron = services.get_cron(cron_id)
    except services.CronError as exc:
        messages.error(request, str(exc))
        return HttpResponseRedirect(reverse("crons:list"))

    ctx = {"cron_obj": cron}
    # ?partial=1 → solo el bloque detail (para modal).
    if request.GET.get("partial") == "1":
        return render(request, "crons/_detail_partial.html", ctx)
    return render(request, "crons/detail.html", ctx)


# === Crear ===

@require_http_methods(["GET", "POST"])
def cron_create(request):
    if (redir := _require_auth(request)):
        return redir

    if request.method == "POST":
        form = CronForm(request.POST)
        if form.is_valid():
            try:
                cron = services.create_cron(
                    name=form.cleaned_data["name"],
                    slug=form.cleaned_data["slug"],
                    description=form.cleaned_data["description"],
                    cron_expression=form.cleaned_data["cron_expression"],
                    timezone=form.cleaned_data["timezone"],
                    target_type=form.cleaned_data["target_type"],
                    target_id=form.cleaned_data["target_id"],
                    inputs=form.cleaned_data["inputs"],
                    enabled=form.cleaned_data["enabled"],
                )
                messages.success(request, f"Cron {cron.name} creado correctamente.")
                if _is_ajax(request):
                    return HttpResponseRedirect(reverse("crons:list"))
                return HttpResponseRedirect(reverse("crons:detail", args=[cron.pk]))
            except services.CronError as exc:
                messages.error(request, str(exc))
                if _is_ajax(request):
                    return render(request, "crons/_form_partial.html", {
                        "form": form, "mode": "create", "cron_obj": None,
                        "action": reverse("crons:create"),
                    })
    else:
        form = CronForm()

    ctx = {"form": form, "mode": "create", "cron_obj": None,
           "action": reverse("crons:create")}
    if request.GET.get("partial") == "1":
        return render(request, "crons/_form_partial.html", ctx)
    return render(request, "crons/form.html", ctx)


# === Editar ===

@require_http_methods(["GET", "POST"])
def cron_edit(request, cron_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        cron = services.get_cron(cron_id)
    except services.CronError as exc:
        messages.error(request, str(exc))
        return HttpResponseRedirect(reverse("crons:list"))

    if request.method == "POST":
        form = CronForm(request.POST, instance=cron)
        if form.is_valid():
            try:
                cron = services.update_cron(
                    cron,
                    name=form.cleaned_data["name"],
                    slug=form.cleaned_data["slug"],
                    description=form.cleaned_data["description"],
                    cron_expression=form.cleaned_data["cron_expression"],
                    timezone=form.cleaned_data["timezone"],
                    target_type=form.cleaned_data["target_type"],
                    target_id=form.cleaned_data["target_id"],
                    inputs=form.cleaned_data["inputs"],
                    enabled=form.cleaned_data["enabled"],
                )
                messages.success(request, f"Cron {cron.name} actualizado.")
                if _is_ajax(request):
                    return HttpResponseRedirect(reverse("crons:list"))
                return HttpResponseRedirect(reverse("crons:detail", args=[cron.pk]))
            except services.CronError as exc:
                messages.error(request, str(exc))
                if _is_ajax(request):
                    return render(request, "crons/_form_partial.html", {
                        "form": form, "mode": "edit", "cron_obj": cron,
                        "action": reverse("crons:edit", args=[cron.pk]),
                    })
    else:
        form = CronForm(instance=cron)

    ctx = {"form": form, "mode": "edit", "cron_obj": cron,
           "action": reverse("crons:edit", args=[cron.pk])}
    if request.GET.get("partial") == "1":
        return render(request, "crons/_form_partial.html", ctx)
    return render(request, "crons/form.html", ctx)


# === Eliminar (soft) ===

@require_http_methods(["POST"])
def cron_delete(request, cron_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        cron = services.get_cron(cron_id)
        services.delete_cron(cron)
        messages.success(request, f"Cron {cron.name} eliminado (soft delete).")
    except services.CronError as exc:
        messages.error(request, str(exc))

    return HttpResponseRedirect(reverse("crons:list"))


# === Toggle enabled/disabled ===

@require_http_methods(["POST"])
def cron_toggle(request, cron_id: str):
    """Toggle del flag enabled True <-> False vía AJAX."""
    if (redir := _require_auth(request)):
        return redir

    try:
        cron = services.get_cron(cron_id)
        new_enabled = services.toggle_cron_enabled(cron)
        label = "habilitado" if new_enabled else "deshabilitado"
        messages.success(request, f"Cron {cron.name} {label}.")
    except services.CronError as exc:
        messages.error(request, str(exc))

    return HttpResponseRedirect(reverse("crons:list"))
