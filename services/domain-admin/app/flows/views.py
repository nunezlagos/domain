"""views (controllers) del mantenedor de Flows.

================================ SDD ================================
HU: Como administrador de la organización quiero un mantenedor de Flows
    para crear, listar, editar, activar/desactivar y dar de baja (soft)
    los flows (DAGs declarativos) que la organización ejecuta, y ver el
    historial de versiones (snapshots inmutables) de cada flow.

Criterios de aceptación:
  1. El listado muestra los flows NO eliminados, paginado, con búsqueda
     server-side por nombre/slug/descripción.
  2. El listado se auto-refresca solo cuando la BD cambia (señal
     count + max(updated_at)), no con polling ciego.
  3. Crear/editar se hacen en un modal AJAX (?partial=1 + header
     X-Requested-With: fetch). Form inválido re-renderiza el modal con
     errores; submit OK recarga el listado.
  4. El detalle se abre en modal (?partial=1) e incluye la lista
     READ-ONLY de versiones (flow_versions) del flow; sobre las versiones
     no hay CRUD.
  5. Toggle alterna is_active (habilitado <-> deshabilitado, POST).
     Eliminar es soft-delete (POST): marca deleted_at + is_active=false,
     NO borra la fila.
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
from .forms import FlowForm, FlowSearchForm


def _require_auth(request):
    """Redirect a /login/ si no está autenticado."""
    if not request.session.get("authenticated"):
        return HttpResponseRedirect("/login/")
    return None


def _is_ajax(request) -> bool:
    """El front manda el header literal 'fetch' (no 'XMLHttpRequest')."""
    return request.headers.get("X-Requested-With") == "fetch"


# === Listado ===

def flow_list(request):
    if (redir := _require_auth(request)):
        return redir

    search_form = FlowSearchForm(request.GET or None)
    search = ""
    if search_form.is_valid():
        search = search_form.cleaned_data.get("q", "").strip()

    page_num = int(request.GET.get("page", 1) or 1)
    per_page = 20

    data = services.list_flows(search=search, page=page_num, per_page=per_page)

    # ?fragment=table → solo tabla + paginación (sin base/layout), para
    # el refresh on-change y la paginación/búsqueda AJAX.
    if request.GET.get("fragment") == "table":
        return render(request, "flows/_table_partial.html", {
            "flows": data["flows"],
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
    return render(request, "flows/list.html", {
        "flows": data["flows"],
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


def flow_list_signal(request):
    """Señal de cambios (JSON) para refresh on-change. Query barata."""
    if (redir := _require_auth(request)):
        return redir
    return JsonResponse(services.get_list_signal())


# === Detalle ===

def flow_detail(request, flow_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        flow = services.get_flow(flow_id)
    except services.FlowError as exc:
        messages.error(request, str(exc))
        return HttpResponseRedirect(reverse("flows:list"))

    ctx = {
        "flow_obj": flow,
        "flow_versions": services.get_flow_versions(flow),
    }
    # ?partial=1 → solo el bloque detail (para modal).
    if request.GET.get("partial") == "1":
        return render(request, "flows/_detail_partial.html", ctx)
    return render(request, "flows/detail.html", ctx)


# === Crear ===

@require_http_methods(["GET", "POST"])
def flow_create(request):
    if (redir := _require_auth(request)):
        return redir

    if request.method == "POST":
        form = FlowForm(request.POST)
        if form.is_valid():
            try:
                flow = services.create_flow(
                    organization_id=form.cleaned_data["organization_id"],
                    name=form.cleaned_data["name"],
                    slug=form.cleaned_data["slug"],
                    description=form.cleaned_data["description"],
                    spec=form.cleaned_data["spec"],
                    is_active=form.cleaned_data["is_active"],
                    deterministic_replay=form.cleaned_data["deterministic_replay"],
                    seed_managed=form.cleaned_data["seed_managed"],
                    seed_version=form.cleaned_data["seed_version"],
                )
                messages.success(request, f"Flow {flow.name} creado correctamente.")
                if _is_ajax(request):
                    return HttpResponseRedirect(reverse("flows:list"))
                return HttpResponseRedirect(reverse("flows:detail", args=[flow.pk]))
            except services.FlowError as exc:
                messages.error(request, str(exc))
                if _is_ajax(request):
                    return render(request, "flows/_form_partial.html", {
                        "form": form, "mode": "create", "flow_obj": None,
                        "action": reverse("flows:create"),
                    })
    else:
        form = FlowForm()

    ctx = {"form": form, "mode": "create", "flow_obj": None,
           "action": reverse("flows:create")}
    if request.GET.get("partial") == "1":
        return render(request, "flows/_form_partial.html", ctx)
    return render(request, "flows/form.html", ctx)


# === Editar ===

@require_http_methods(["GET", "POST"])
def flow_edit(request, flow_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        flow = services.get_flow(flow_id)
    except services.FlowError as exc:
        messages.error(request, str(exc))
        return HttpResponseRedirect(reverse("flows:list"))

    if request.method == "POST":
        form = FlowForm(request.POST, instance=flow)
        if form.is_valid():
            try:
                flow = services.update_flow(
                    flow,
                    name=form.cleaned_data["name"],
                    slug=form.cleaned_data["slug"],
                    description=form.cleaned_data["description"],
                    spec=form.cleaned_data["spec"],
                    is_active=form.cleaned_data["is_active"],
                    deterministic_replay=form.cleaned_data["deterministic_replay"],
                    seed_managed=form.cleaned_data["seed_managed"],
                    seed_version=form.cleaned_data["seed_version"],
                )
                messages.success(request, f"Flow {flow.name} actualizado.")
                if _is_ajax(request):
                    return HttpResponseRedirect(reverse("flows:list"))
                return HttpResponseRedirect(reverse("flows:detail", args=[flow.pk]))
            except services.FlowError as exc:
                messages.error(request, str(exc))
                if _is_ajax(request):
                    return render(request, "flows/_form_partial.html", {
                        "form": form, "mode": "edit", "flow_obj": flow,
                        "action": reverse("flows:edit", args=[flow.pk]),
                    })
    else:
        form = FlowForm(instance=flow)

    ctx = {"form": form, "mode": "edit", "flow_obj": flow,
           "action": reverse("flows:edit", args=[flow.pk])}
    if request.GET.get("partial") == "1":
        return render(request, "flows/_form_partial.html", ctx)
    return render(request, "flows/form.html", ctx)


# === Eliminar (soft) ===

@require_http_methods(["POST"])
def flow_delete(request, flow_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        flow = services.get_flow(flow_id)
        services.delete_flow(flow)
        messages.success(request, f"Flow {flow.name} eliminado (soft delete).")
    except services.FlowError as exc:
        messages.error(request, str(exc))

    return HttpResponseRedirect(reverse("flows:list"))


# === Toggle is_active ===

@require_http_methods(["POST"])
def flow_toggle(request, flow_id: str):
    """Toggle de is_active (habilitado <-> deshabilitado) vía AJAX."""
    if (redir := _require_auth(request)):
        return redir

    try:
        flow = services.get_flow(flow_id)
        is_active = services.toggle_flow_status(flow)
        label = "activado" if is_active else "desactivado"
        messages.success(request, f"Flow {flow.name} {label}.")
    except services.FlowError as exc:
        messages.error(request, str(exc))

    return HttpResponseRedirect(reverse("flows:list"))
