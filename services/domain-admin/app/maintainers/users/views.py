"""Views del mantenedor de usuarios (migradas a core).

Las 7 vistas estándar (list, signal, detail, create, edit, delete, toggle) las
arma core.views.MaintainerViews. Acá solo:

  1. Se configura la instancia `views` (model/form/service/templates/labels).
  2. Se sobreescriben los hooks específicos de users:
       - _form_payload: mapea role -> role_slug y agrega hashed_password().
       - form_context / detail_context: exponen `user_obj` que los templates
         de users ya consumen.

El guard de auth (require_auth) y la detección AJAX (is_ajax) vienen de
core.auth (antes estaban duplicados como _require_auth/_is_ajax).
"""
from __future__ import annotations

from core.views import MaintainerViews

from . import services
from .forms import UserForm
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
