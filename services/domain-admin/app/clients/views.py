"""views (controllers) del mantenedor de Clientes (mandantes).

================================ SDD ================================
HU: Como administrador de la organización quiero un mantenedor de
    Clientes (mandantes) para crear, listar, editar, activar/desactivar
    y dar de baja (soft) las cuentas/empresas contraparte que la
    organización gestiona.

Criterios de aceptación:
  1. El listado muestra los clientes NO eliminados, paginado, con
     búsqueda server-side por nombre/slug/tax_id/email.
  2. El listado se auto-refresca solo cuando la BD cambia (señal
     count + max(updated_at)), no con polling ciego.
  3. Crear/editar se hacen en un modal AJAX (?partial=1 + header
     X-Requested-With: fetch). Form inválido re-renderiza el modal con
     errores; submit OK recarga el listado.
  4. El detalle se abre en modal (?partial=1).
  5. Toggle alterna active <-> inactive (POST). Eliminar es soft-delete
     (POST): marca deleted_at + status=archived, NO borra la fila.
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
from .forms import ClientForm, ClientSearchForm


def _require_auth(request):
    """Redirect a /login/ si no está autenticado."""
    if not request.session.get("authenticated"):
        return HttpResponseRedirect("/login/")
    return None


def _is_ajax(request) -> bool:
    """El front manda el header literal 'fetch' (no 'XMLHttpRequest')."""
    return request.headers.get("X-Requested-With") == "fetch"


# === Listado ===

def client_list(request):
    if (redir := _require_auth(request)):
        return redir

    search_form = ClientSearchForm(request.GET or None)
    search = ""
    if search_form.is_valid():
        search = search_form.cleaned_data.get("q", "").strip()

    page_num = int(request.GET.get("page", 1) or 1)
    per_page = 20

    data = services.list_clients(search=search, page=page_num, per_page=per_page)

    # ?fragment=table → solo tabla + paginación (sin base/layout), para
    # el refresh on-change y la paginación/búsqueda AJAX.
    if request.GET.get("fragment") == "table":
        return render(request, "clients/_table_partial.html", {
            "clients": data["clients"],
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
    return render(request, "clients/list.html", {
        "clients": data["clients"],
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


def client_list_signal(request):
    """Señal de cambios (JSON) para refresh on-change. Query barata."""
    if (redir := _require_auth(request)):
        return redir
    return JsonResponse(services.get_list_signal())


# === Detalle ===

def client_detail(request, client_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        client = services.get_client(client_id)
    except services.ClientError as exc:
        messages.error(request, str(exc))
        return HttpResponseRedirect(reverse("clients:list"))

    ctx = {"client_obj": client}
    # ?partial=1 → solo el bloque detail (para modal).
    if request.GET.get("partial") == "1":
        return render(request, "clients/_detail_partial.html", ctx)
    return render(request, "clients/detail.html", ctx)


# === Crear ===

@require_http_methods(["GET", "POST"])
def client_create(request):
    if (redir := _require_auth(request)):
        return redir

    if request.method == "POST":
        form = ClientForm(request.POST)
        if form.is_valid():
            try:
                client = services.create_client(
                    organization_id=form.cleaned_data["organization_id"],
                    name=form.cleaned_data["name"],
                    slug=form.cleaned_data["slug"],
                    tax_id=form.cleaned_data["tax_id"],
                    contact_email=form.cleaned_data["contact_email"],
                    contact_phone=form.cleaned_data["contact_phone"],
                    address=form.cleaned_data["address"],
                    status=form.cleaned_data["status"],
                )
                messages.success(request, f"Cliente {client.name} creado correctamente.")
                if _is_ajax(request):
                    return HttpResponseRedirect(reverse("clients:list"))
                return HttpResponseRedirect(reverse("clients:detail", args=[client.pk]))
            except services.ClientError as exc:
                messages.error(request, str(exc))
                if _is_ajax(request):
                    return render(request, "clients/_form_partial.html", {
                        "form": form, "mode": "create", "client_obj": None,
                        "action": reverse("clients:create"),
                    })
    else:
        form = ClientForm()

    ctx = {"form": form, "mode": "create", "client_obj": None,
           "action": reverse("clients:create")}
    if request.GET.get("partial") == "1":
        return render(request, "clients/_form_partial.html", ctx)
    return render(request, "clients/form.html", ctx)


# === Editar ===

@require_http_methods(["GET", "POST"])
def client_edit(request, client_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        client = services.get_client(client_id)
    except services.ClientError as exc:
        messages.error(request, str(exc))
        return HttpResponseRedirect(reverse("clients:list"))

    if request.method == "POST":
        form = ClientForm(request.POST, instance=client)
        if form.is_valid():
            try:
                client = services.update_client(
                    client,
                    name=form.cleaned_data["name"],
                    slug=form.cleaned_data["slug"],
                    tax_id=form.cleaned_data["tax_id"],
                    contact_email=form.cleaned_data["contact_email"],
                    contact_phone=form.cleaned_data["contact_phone"],
                    address=form.cleaned_data["address"],
                    status=form.cleaned_data["status"],
                )
                messages.success(request, f"Cliente {client.name} actualizado.")
                if _is_ajax(request):
                    return HttpResponseRedirect(reverse("clients:list"))
                return HttpResponseRedirect(reverse("clients:detail", args=[client.pk]))
            except services.ClientError as exc:
                messages.error(request, str(exc))
                if _is_ajax(request):
                    return render(request, "clients/_form_partial.html", {
                        "form": form, "mode": "edit", "client_obj": client,
                        "action": reverse("clients:edit", args=[client.pk]),
                    })
    else:
        form = ClientForm(instance=client)

    ctx = {"form": form, "mode": "edit", "client_obj": client,
           "action": reverse("clients:edit", args=[client.pk])}
    if request.GET.get("partial") == "1":
        return render(request, "clients/_form_partial.html", ctx)
    return render(request, "clients/form.html", ctx)


# === Eliminar (soft) ===

@require_http_methods(["POST"])
def client_delete(request, client_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        client = services.get_client(client_id)
        services.delete_client(client)
        messages.success(request, f"Cliente {client.name} eliminado (soft delete).")
    except services.ClientError as exc:
        messages.error(request, str(exc))

    return HttpResponseRedirect(reverse("clients:list"))


# === Toggle active/inactive ===

@require_http_methods(["POST"])
def client_toggle(request, client_id: str):
    """Toggle de status active <-> inactive vía AJAX."""
    if (redir := _require_auth(request)):
        return redir

    try:
        client = services.get_client(client_id)
        new_status = services.toggle_client_status(client)
        labels = {"active": "activado", "inactive": "desactivado", "archived": "archivado"}
        label = labels.get(new_status, new_status)
        messages.success(request, f"Cliente {client.name} {label}.")
    except services.ClientError as exc:
        messages.error(request, str(exc))

    return HttpResponseRedirect(reverse("clients:list"))
