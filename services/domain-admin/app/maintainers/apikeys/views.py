"""Views del mantenedor de API Keys (migradas a core).

Las 7 vistas estandar (list, signal, detail, create, edit, delete, toggle) las
arma core.views.MaintainerViews. Aqui solo se especializa lo propio de keys:

  - _form_payload: separa name/expires_at/status del `user` (que el service de
    create necesita como instancia User, no como id).
  - do_create: resuelve el User dueño, llama create_api_key (que devuelve
    (obj, secreto)) y mete el secreto en el messages.success — se muestra UNA
    sola vez. El core llamaria create_api_key(**payload), pero aqui la firma y el
    valor de retorno difieren, asi que se sobreescribe el hook.
  - form_context / detail_context: exponen `apikey_obj` (lo que los templates
    de apikeys ya consumen) ademas de `object`.

El guard de auth (require_auth) y la deteccion AJAX (is_ajax) vienen de
core.auth (antes estaban duplicados como _require_auth/X-Requested-With inline).
"""
from __future__ import annotations

from django.contrib import messages

from core.views import MaintainerViews
from maintainers.users.models import User

from . import services
from .forms import ApiKeyForm


class ApiKeyViews(MaintainerViews):
    """MaintainerViews especializado para API keys (payload + secreto + ctx)."""

    # --- payload: el service de update espera name/expires_at/status; el de
    #     create ademas necesita el `user` resuelto a instancia (lo hace
    #     do_create). update_api_key NO recibe user (dueño inmutable).
    def _form_payload(self, form) -> dict:
        return {
            "name": form.cleaned_data["name"],
            "expires_at": form.cleaned_data.get("expires_at"),
            "status": form.cleaned_data["status"],
        }

    def do_create(self, form):
        """Crea la key resolviendo el User dueño y exponiendo el secreto.

        create_api_key devuelve (obj, secreto_claro): el secreto se muestra una
        sola vez en el mensaje de exito. Cualquier User.DoesNotExist se traduce
        a ApiKeyError para que la view lo maneje como error de dominio.
        """
        try:
            owner = User.objects.get(pk=form.cleaned_data["user"])
        except User.DoesNotExist as exc:
            raise services.ApiKeyError("El usuario seleccionado no existe.") from exc
        api_key, secret = services.create_api_key(
            user=owner,
            **self._form_payload(form),
        )
        # Se cuela en el flash de create() via mensaje extra; el generico ya
        # agrega "API Key creado correctamente.", aqui sumamos el secreto. Ahora
        # tambien queda visible en el detalle (key_plaintext), asi que no es la
        # unica chance de copiarlo.
        messages.warning(
            self._request,
            f"Secreto de '{api_key.name}' (tambien visible en el detalle): {secret}",
        )
        return api_key

    def do_update(self, instance, form):
        return services.update_api_key(instance, **self._form_payload(form))

    # --- create/edit guardan el request para que do_create pueda emitir el
    #     mensaje con el secreto (el hook do_create no recibe request).
    def create(self, request):
        self._request = request
        return super().create(request)

    def edit(self, request, **kwargs):
        self._request = request
        return super().edit(request, **kwargs)

    # --- contextos: los templates de apikeys usan `apikey_obj` (no `object`).
    def list_context(self, data: dict, search: str) -> dict:
        ctx = super().list_context(data, search)
        ctx["page_title"] = "API Keys"
        return ctx

    def form_context(self, form, mode: str, instance, action: str) -> dict:
        return {
            "form": form,
            "mode": mode,
            "apikey_obj": instance,
            "object": instance,
            "action": action,
        }

    def detail_context(self, instance) -> dict:
        return {"apikey_obj": instance, "object": instance}


# Instancia que cablea todo. list_key="api_keys" -> el template recibe la lista
# bajo `api_keys`. id_kwarg="apikey_id" -> casa con <uuid:apikey_id> de las URLs.
views = ApiKeyViews(
    app_name="apikeys",
    model=services.ApiKey,
    form_class=ApiKeyForm,
    service=services,
    templates="apikeys",
    search_fields=("name", "key_prefix"),
    entity_label="API Key",
    id_kwarg="apikey_id",
    list_key="api_keys",
    per_page=20,
    search_param="q",
)
