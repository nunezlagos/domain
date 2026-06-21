"""HU-48.1: views (controllers en MVC).

Cada view:
1. Recibe request HTTP
2. Llama al service correspondiente (lógica de negocio)
3. Renderiza template (o redirige)

Mensajes de feedback via Django messages framework.
"""
from __future__ import annotations

from django.contrib import messages
from django.core.paginator import Paginator
from django.http import HttpResponseRedirect, JsonResponse
from django.shortcuts import render
from django.urls import path, reverse
from django.views.decorators.http import require_http_methods

from . import services
from .forms import UserForm, UserRoleAssignForm, UserSearchForm
from .models import User


def _require_auth(request):
    """Redirect a /login/ si no está autenticado."""
    if not request.session.get("authenticated"):
        return HttpResponseRedirect("/login/")
    return None


# === Listado ===

def user_list(request):
    if (redir := _require_auth(request)):
        return redir

    search_form = UserSearchForm(request.GET or None)
    search = ""
    if search_form.is_valid():
        search = search_form.cleaned_data.get("q", "").strip()

    page_num = int(request.GET.get("page", 1) or 1)
    per_page = 20

    data = services.list_users(search=search, page=page_num, per_page=per_page)

    # HU-48.2: ?fragment=table → solo tabla + paginación (sin base/layout).
    # Para auto-refresh vía polling.
    if request.GET.get("fragment") == "table":
        return render(request, "users/_table_partial.html", {
            "users": data["users"],
            "total": data["total"],
            "page": data["page"],
            "per_page": data["per_page"],
            "total_pages": data["total_pages"],
            "has_next": data["has_next"],
            "has_prev": data["has_prev"],
            "search": search,
        })

    # Señal inicial: se embebe en el HTML para que el front parta con la
    # versión exacta del render (evita perder un cambio entre render y 1er poll).
    sig = services.get_list_signal()
    return render(request, "users/list.html", {
        "users": data["users"],
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


def user_list_signal(request):
    """HU-48.2: señal de cambios (JSON) para refresh on-change.

    Reemplaza el polling ciego de la tabla: el front pega acá (query barata),
    compara con su última señal y solo refresca la tabla si cambió.
    """
    if (redir := _require_auth(request)):
        return redir
    return JsonResponse(services.get_list_signal())


# === Detalle ===

def user_detail(request, user_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        user = services.get_user(user_id)
    except services.UserError as exc:
        messages.error(request, str(exc))
        if request.headers.get("X-Requested-With") == "fetch":
            return HttpResponseRedirect(reverse("users:list"))
        return HttpResponseRedirect(reverse("users:list"))

    user_roles = services.get_user_roles(user)
    available_roles = services.list_available_roles()

    ctx = {
        "user_obj": user,
        "user_roles": user_roles,
        "available_roles": available_roles,
        "assign_form": UserRoleAssignForm(user=user),
    }
    # ?partial=1 → solo el bloque detail (para modal)
    if request.GET.get("partial") == "1":
        return render(request, "users/_detail_partial.html", ctx)
    return render(request, "users/detail.html", ctx)


# === Crear ===

@require_http_methods(["GET", "POST"])
def user_create(request):
    if (redir := _require_auth(request)):
        return redir

    if request.method == "POST":
        form = UserForm(request.POST)
        if form.is_valid():
            try:
                user = services.create_user(
                    email=form.cleaned_data["email"],
                    name=form.cleaned_data["name"],
                    role_slug=form.cleaned_data["role"],
                    status=form.cleaned_data["status"],
                    hashed_password=form.hashed_password(),
                )
                messages.success(request, f"Usuario {user.email} creado correctamente.")
                # AJAX: devolver redirect header para que el JS haga reload
                if request.headers.get("X-Requested-With") == "fetch":
                    return HttpResponseRedirect(reverse("users:list"))
                return HttpResponseRedirect(reverse("users:detail", args=[user.pk]))
            except services.UserError as exc:
                messages.error(request, str(exc))
                # AJAX: re-renderizar form con errores en modal
                if request.headers.get("X-Requested-With") == "fetch":
                    return render(request, "users/_form_partial.html", {
                        "form": form, "mode": "create", "user_obj": None,
                        "action": reverse("users:create"),
                    })
    else:
        form = UserForm()

    # ?partial=1 → solo el form para inyectar en modal
    ctx = {"form": form, "mode": "create", "user_obj": None,
           "action": reverse("users:create")}
    if request.GET.get("partial") == "1":
        return render(request, "users/_form_partial.html", ctx)
    return render(request, "users/form.html", ctx)


# === Editar ===

@require_http_methods(["GET", "POST"])
def user_edit(request, user_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        user = services.get_user(user_id)
    except services.UserError as exc:
        messages.error(request, str(exc))
        if request.headers.get("X-Requested-With") == "fetch":
            return HttpResponseRedirect(reverse("users:list"))
        return HttpResponseRedirect(reverse("users:list"))

    if request.method == "POST":
        form = UserForm(request.POST, instance=user)
        if form.is_valid():
            try:
                user = services.update_user(
                    user,
                    email=form.cleaned_data["email"],
                    name=form.cleaned_data["name"],
                    role_slug=form.cleaned_data["role"],
                    status=form.cleaned_data["status"],
                    hashed_password=form.hashed_password(),
                )
                messages.success(request, f"Usuario {user.email} actualizado.")
                if request.headers.get("X-Requested-With") == "fetch":
                    return HttpResponseRedirect(reverse("users:list"))
                return HttpResponseRedirect(reverse("users:detail", args=[user.pk]))
            except services.UserError as exc:
                messages.error(request, str(exc))
                if request.headers.get("X-Requested-With") == "fetch":
                    return render(request, "users/_form_partial.html", {
                        "form": form, "mode": "edit", "user_obj": user,
                        "action": reverse("users:edit", args=[user.pk]),
                    })
    else:
        form = UserForm(instance=user)

    # ?partial=1 → solo el form para inyectar en modal
    ctx = {"form": form, "mode": "edit", "user_obj": user,
           "action": reverse("users:edit", args=[user.pk])}
    if request.GET.get("partial") == "1":
        return render(request, "users/_form_partial.html", ctx)
    return render(request, "users/form.html", ctx)


# === Eliminar ===

@require_http_methods(["POST"])
def user_delete(request, user_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        user = services.get_user(user_id)
        services.delete_user(user)
        messages.success(request, f"Usuario {user.email} eliminado (soft delete).")
    except services.UserError as exc:
        messages.error(request, str(exc))

    return HttpResponseRedirect(reverse("users:list"))


# === Roles (asignar / revocar) ===

@require_http_methods(["POST"])
def role_assign(request, user_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        user = services.get_user(user_id)
        form = UserRoleAssignForm(request.POST, user=user)
        if form.is_valid():
            role = form.cleaned_data["role"]
            services.assign_role(user, role.pk)
            messages.success(request, f"Rol '{role.slug}' asignado a {user.email}.")
        else:
            messages.error(request, "Formulario inválido.")
    except services.UserError as exc:
        messages.error(request, str(exc))
    except Exception as exc:
        messages.error(request, f"Error: {exc}")

    return HttpResponseRedirect(reverse("users:detail", args=[user_id]))


@require_http_methods(["POST"])
def role_revoke(request, user_id: str, role_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        user = services.get_user(user_id)
        if services.revoke_role(user, role_id):
            messages.success(request, "Rol revocado.")
        else:
            messages.warning(request, "El rol no estaba asignado.")
    except services.UserError as exc:
        messages.error(request, str(exc))

    return HttpResponseRedirect(reverse("users:detail", args=[user_id]))


# === Toggle active/suspended (HU-48.2) ===

@require_http_methods(["POST"])
def user_toggle(request, user_id: str):
    """Toggle de status active<->suspended via AJAX."""
    if (redir := _require_auth(request)):
        return redir

    try:
        user = services.get_user(user_id)
        new_status = services.toggle_user_status(user)
        labels = {"active": "activado", "suspended": "suspendido", "pending": "marcado pendiente", "revoked": "revocado"}
        label = labels.get(new_status, new_status)
        messages.success(request, f"Usuario {user.email} {label}.")
    except services.UserError as exc:
        messages.error(request, str(exc))

    # AJAX: redirige al list; el JS hace reload para mostrar el cambio
    return HttpResponseRedirect(reverse("users:list"))


# === URLs (MVC routing) ===
urlpatterns = [
    path("", user_list, name="list"),
    path("nuevo/", user_create, name="create"),
    path("<uuid:user_id>/", user_detail, name="detail"),
    path("<uuid:user_id>/editar/", user_edit, name="edit"),
    path("<uuid:user_id>/eliminar/", user_delete, name="delete"),
    path("<uuid:user_id>/roles/asignar/", role_assign, name="role_assign"),
    path("<uuid:user_id>/roles/<uuid:role_id>/revocar/", role_revoke, name="role_revoke"),
]