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

from django.contrib import messages
from django.http import HttpResponseRedirect
from django.urls import reverse
from django.views.decorators.http import require_http_methods

from core.auth import require_auth
from core.views import MaintainerViews

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
            "name": form.cleaned_data["name"],
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
