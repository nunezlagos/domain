"""Views del mantenedor de usuarios (migradas a core).

Las 7 vistas estándar (list, signal, detail, create, edit, delete, toggle) las
arma core.views.MaintainerViews. Acá solo:

  1. Se configura la instancia `views` (model/form/service/templates/labels).
  2. Se sobreescriben los hooks específicos de users:
       - _form_payload: mapea role -> role_slug y agrega hashed_password().
       - form_context / detail_context: exponen `user_obj` (+ roles en detail)
         que los templates de users ya consumen.
  3. Se agregan las 2 vistas propias del dominio roles (asignar / revocar),
     que NO son parte del CRUD estándar.

El guard de auth (require_auth) y la detección AJAX (is_ajax) vienen de
core.auth (antes estaban duplicados como _require_auth/_is_ajax).
"""
from __future__ import annotations

import uuid

from django.contrib import messages
from django.http import HttpResponseRedirect
from django.shortcuts import render
from django.urls import reverse
from django.utils import timezone
from django.views.decorators.http import require_http_methods

from core.auth import require_auth
from core.views import MaintainerViews
from maintainers.apikeys.models import ApiKey
from maintainers.apikeys.services import list_api_keys

from . import services
from .forms import UserForm, UserRoleAssignForm
from .models import User


class UserViews(MaintainerViews):
    """MaintainerViews especializado para users (context keys + payload + msgs)."""

    # --- payload del service: el form trae role/password; el service espera
    #     role_slug + hashed_password y NO conoce password/password_confirm.
    def _form_payload(self, form) -> dict:
        return {
            "email": form.cleaned_data["email"],
            "name": form.composed_name(),
            "role_slug": form.cleaned_data["role"],
            "status": form.cleaned_data["status"],
            "hashed_password": form.hashed_password(),
        }

    # --- contextos: los templates de users usan `user_obj` (no `object`).
    def list_context(self, data: dict, search: str) -> dict:
        ctx = super().list_context(data, search)
        ctx["page_title"] = "Usuarios"
        return ctx

    def form_context(self, form, mode: str, instance, action: str) -> dict:
        return {
            "form": form,
            "mode": mode,
            "user_obj": instance,
            "object": instance,
            "action": action,
        }

    def detail_context(self, instance) -> dict:
        return {
            "user_obj": instance,
            "object": instance,
            "user_roles": services.get_user_roles(instance),
            "available_roles": services.list_available_roles(),
            "assign_form": UserRoleAssignForm(user=instance),
            # API keys del usuario, para la sección "API Keys" del modal de detalle.
            "api_keys": list(
                ApiKey.objects.filter(user_id=instance.pk).order_by("-created_at")
            ),
        }


# Instancia que cablea todo. list_key="users" -> el template recibe la lista
# bajo `users`. id_kwarg="user_id" -> casa con <uuid:user_id> de las URLs.
views = UserViews(
    app_name="users",
    model=User,
    form_class=UserForm,
    service=services,
    templates="users",
    search_fields=("email", "name"),
    entity_label="Usuario",
    id_kwarg="user_id",
    list_key="users",
    per_page=20,
    search_param="q",
)


# === Vistas propias del dominio roles (fuera del CRUD estándar) ===

@require_http_methods(["POST"])
def role_assign(request, user_id: str):
    if (redir := require_auth(request)):
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
    except Exception as exc:  # noqa: BLE001 — feedback al usuario, no swallow silencioso
        messages.error(request, f"Error: {exc}")

    return HttpResponseRedirect(reverse("users:detail", args=[user_id]))


@require_http_methods(["POST"])
def role_revoke(request, user_id: str, role_id: str):
    if (redir := require_auth(request)):
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


# === Consolidación API Keys + Invitaciones dentro del mantenedor de Usuarios ===

@require_http_methods(["GET"])
def apikeys_modal(request):
    """Listado COMPACTO de API Keys para el modal "Gestionar API Keys".

    Reusa apikeys.services.list_api_keys (search/paginación del MaintainerService)
    y renderiza un partial chico. Las acciones por fila (editar/revocar/crear)
    apuntan al mantenedor existente (/api-keys/) vía data-base-url, así que el
    submit lo maneja modals.js contra las rutas estándar de apikeys.
    """
    if (redir := require_auth(request)):
        return redir

    data = list_api_keys(search="", page=1, per_page=100)
    return render(
        request,
        "users/_apikeys_modal.html",
        {"api_keys": data["api_keys"], "total": data["total"]},
    )


@require_http_methods(["GET"])
def invite_preview(request, user_id: str):
    """Preview del email de invitación (PREVIEW-ONLY, sin SMTP ni persistencia).

    Renderiza el email HTML real (templates/emails/invitation.html) con un token
    de enrollment generado al vuelo y expiración a 7 días, dentro de un modal con
    nota de "envío pendiente" y botón "Copiar link".

    IMPORTANTE: NO se persiste en auth_invitations (se evita el FK
    invited_by_user_id). El registro de la invitación y el envío real por SMTP
    quedan como trabajo futuro: acá solo se muestra cómo se vería el correo.
    """
    if (redir := require_auth(request)):
        return redir

    try:
        user = services.get_user(user_id)
    except services.UserError as exc:
        messages.error(request, str(exc))
        return HttpResponseRedirect(reverse("users:list"))

    token = uuid.uuid4()
    expires_at = timezone.now() + timezone.timedelta(days=7)
    enrollment_url = request.build_absolute_uri(f"/enroll/{token}/")

    ctx = {
        "user_obj": user,
        "user_name": user.display_name,
        "user_email": user.email,
        "user_role": user.role,
        "enrollment_url": enrollment_url,
        "token": token,
        "expires_at": expires_at,
        "expires_days": 7,
    }
    return render(request, "users/_invite_preview.html", ctx)
