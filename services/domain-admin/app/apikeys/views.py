"""HU-API.1 — Mantenedor de API Keys (views / controllers MVC).

HISTORIA DE USUARIO
    Como administrador de la plataforma quiero gestionar las API Keys
    (auth_api_keys) de los usuarios desde el panel, para poder crear,
    revocar y auditar credenciales de acceso programático sin tocar la BD.

CRITERIOS DE ACEPTACIÓN
    1. El listado muestra todas las API keys (nombre, prefijo, dueño,
       estado, expiración) con búsqueda server-side por nombre/prefijo
       y paginación. Se refresca on-change (señal count+max(updated_at)).
    2. Crear una key genera el secreto una sola vez, persiste prefix+hash,
       y el secreto en claro se muestra al admin solo en esa respuesta.
    3. Editar permite cambiar nombre, expiración y estado; NO reasigna el
       dueño ni regenera el secreto.
    4. Eliminar es soft-delete (revoked_at + status='revoked'); el registro
       se conserva para auditoría.
    5. Toggle alterna active <-> revoked.
    6. Toda operación exige sesión autenticada (redirect a /login/ si no).
    7. Las vistas create/edit soportan ?partial=1 (form en modal) y AJAX
       (X-Requested-With: fetch); detail soporta ?partial=1; delete/toggle
       aceptan POST.

Cada view: recibe HTTP → llama al service → renderiza/redirige. Mensajes de
feedback via Django messages framework.
"""
from __future__ import annotations

from django.contrib import messages
from django.http import HttpResponseRedirect, JsonResponse
from django.shortcuts import render
from django.urls import reverse
from django.views.decorators.http import require_http_methods

from maintainers.users.models import User

from . import services
from .forms import ApiKeyForm, ApiKeySearchForm


def _require_auth(request):
    """Redirect a /login/ si no está autenticado."""
    if not request.session.get("authenticated"):
        return HttpResponseRedirect("/login/")
    return None


# === Listado ===

def apikey_list(request):
    if (redir := _require_auth(request)):
        return redir

    search_form = ApiKeySearchForm(request.GET or None)
    search = ""
    if search_form.is_valid():
        search = search_form.cleaned_data.get("q", "").strip()

    page_num = int(request.GET.get("page", 1) or 1)
    per_page = 20

    data = services.list_api_keys(search=search, page=page_num, per_page=per_page)

    # ?fragment=table → solo tabla + paginación (sin base/layout) para refresh.
    if request.GET.get("fragment") == "table":
        return render(request, "apikeys/_table_partial.html", {
            "api_keys": data["api_keys"],
            "total": data["total"],
            "page": data["page"],
            "per_page": data["per_page"],
            "total_pages": data["total_pages"],
            "has_next": data["has_next"],
            "has_prev": data["has_prev"],
            "search": search,
        })

    # Señal inicial embebida en el HTML: el front parte con la versión exacta
    # del render (evita perder un cambio entre render y primer poll).
    sig = services.get_list_signal()
    return render(request, "apikeys/list.html", {
        "api_keys": data["api_keys"],
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


def apikey_list_signal(request):
    """Señal de cambios (JSON) para refresh on-change.

    Reemplaza el polling ciego: el front pega acá (query barata), compara con
    su última señal y solo refresca la tabla si cambió.
    """
    if (redir := _require_auth(request)):
        return redir
    return JsonResponse(services.get_list_signal())


# === Detalle ===

def apikey_detail(request, apikey_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        api_key = services.get_api_key(apikey_id)
    except services.ApiKeyError as exc:
        messages.error(request, str(exc))
        return HttpResponseRedirect(reverse("apikeys:list"))

    ctx = {"apikey_obj": api_key}
    if request.GET.get("partial") == "1":
        return render(request, "apikeys/_detail_partial.html", ctx)
    return render(request, "apikeys/detail.html", ctx)


# === Crear ===

@require_http_methods(["GET", "POST"])
def apikey_create(request):
    if (redir := _require_auth(request)):
        return redir

    if request.method == "POST":
        form = ApiKeyForm(request.POST)
        if form.is_valid():
            try:
                owner = User.objects.get(pk=form.cleaned_data["user"])
                api_key, secret = services.create_api_key(
                    user=owner,
                    name=form.cleaned_data["name"],
                    expires_at=form.cleaned_data.get("expires_at"),
                    status=form.cleaned_data["status"],
                )
                # El secreto en claro se muestra una sola vez.
                messages.success(
                    request,
                    f"API Key '{api_key.name}' creada. Secreto (cópialo ahora, "
                    f"no se vuelve a mostrar): {secret}",
                )
                if request.headers.get("X-Requested-With") == "fetch":
                    return HttpResponseRedirect(reverse("apikeys:list"))
                return HttpResponseRedirect(reverse("apikeys:detail", args=[api_key.pk]))
            except (services.ApiKeyError, User.DoesNotExist) as exc:
                messages.error(request, str(exc))
                if request.headers.get("X-Requested-With") == "fetch":
                    return render(request, "apikeys/_form_partial.html", {
                        "form": form, "mode": "create", "apikey_obj": None,
                        "action": reverse("apikeys:create"),
                    })
    else:
        form = ApiKeyForm()

    ctx = {"form": form, "mode": "create", "apikey_obj": None,
           "action": reverse("apikeys:create")}
    if request.GET.get("partial") == "1":
        return render(request, "apikeys/_form_partial.html", ctx)
    return render(request, "apikeys/form.html", ctx)


# === Editar ===

@require_http_methods(["GET", "POST"])
def apikey_edit(request, apikey_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        api_key = services.get_api_key(apikey_id)
    except services.ApiKeyError as exc:
        messages.error(request, str(exc))
        return HttpResponseRedirect(reverse("apikeys:list"))

    if request.method == "POST":
        form = ApiKeyForm(request.POST, instance=api_key)
        if form.is_valid():
            try:
                api_key = services.update_api_key(
                    api_key,
                    name=form.cleaned_data["name"],
                    expires_at=form.cleaned_data.get("expires_at"),
                    status=form.cleaned_data["status"],
                )
                messages.success(request, f"API Key '{api_key.name}' actualizada.")
                if request.headers.get("X-Requested-With") == "fetch":
                    return HttpResponseRedirect(reverse("apikeys:list"))
                return HttpResponseRedirect(reverse("apikeys:detail", args=[api_key.pk]))
            except services.ApiKeyError as exc:
                messages.error(request, str(exc))
                if request.headers.get("X-Requested-With") == "fetch":
                    return render(request, "apikeys/_form_partial.html", {
                        "form": form, "mode": "edit", "apikey_obj": api_key,
                        "action": reverse("apikeys:edit", args=[api_key.pk]),
                    })
    else:
        form = ApiKeyForm(instance=api_key)

    ctx = {"form": form, "mode": "edit", "apikey_obj": api_key,
           "action": reverse("apikeys:edit", args=[api_key.pk])}
    if request.GET.get("partial") == "1":
        return render(request, "apikeys/_form_partial.html", ctx)
    return render(request, "apikeys/form.html", ctx)


# === Eliminar (soft delete) ===

@require_http_methods(["POST"])
def apikey_delete(request, apikey_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        api_key = services.get_api_key(apikey_id)
        services.delete_api_key(api_key)
        messages.success(request, f"API Key '{api_key.name}' revocada (soft delete).")
    except services.ApiKeyError as exc:
        messages.error(request, str(exc))

    return HttpResponseRedirect(reverse("apikeys:list"))


# === Toggle active/revoked ===

@require_http_methods(["POST"])
def apikey_toggle(request, apikey_id: str):
    """Toggle de status active<->revoked via AJAX."""
    if (redir := _require_auth(request)):
        return redir

    try:
        api_key = services.get_api_key(apikey_id)
        new_status = services.toggle_api_key_status(api_key)
        labels = {"active": "reactivada", "revoked": "revocada", "expired": "expirada"}
        label = labels.get(new_status, new_status)
        messages.success(request, f"API Key '{api_key.name}' {label}.")
    except services.ApiKeyError as exc:
        messages.error(request, str(exc))

    return HttpResponseRedirect(reverse("apikeys:list"))
