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
from django.http import HttpResponse

from core.auth import require_auth
from core.views import MaintainerViews
from maintainers.users.models import User

from . import services
from .forms import ApiKeyForm
from .models import ApiKey


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

    # --- list con filtros (usuario/estado). Guardamos el request para que
    #     do_list/list_context lean los GET; el resto lo arma core.
    def list(self, request):
        self._list_request = request
        return super().list(request)

    def do_list(self, search: str, page: int) -> dict:
        req = getattr(self, "_list_request", None)
        user = req.GET.get("user") if req else ""
        status = req.GET.get("status") if req else ""
        return services.list_api_keys(
            search=search, page=page, per_page=self.per_page,
            user_id=user or None, status=status or None,
        )

    # --- contextos: los templates de apikeys usan `apikey_obj` (no `object`).
    def list_context(self, data: dict, search: str) -> dict:
        ctx = super().list_context(data, search)
        ctx["page_title"] = "API Keys"
        req = getattr(self, "_list_request", None)
        # Opciones + seleccion actual para el container de filtros.
        ctx["user_options"] = User.objects.filter(status="active").order_by("email")
        ctx["status_options"] = ApiKey.STATUS_CHOICES
        ctx["selected_user"] = req.GET.get("user") if req else ""
        ctx["selected_status"] = req.GET.get("status") if req else ""
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
        # La key en claro para mostrarla/copiarla sale descifrada de
        # key_ciphertext (pgp_sym_decrypt) con fallback al key_plaintext viejo. Se
        # sobreescribe el atributo en la instancia (managed=False, NO persiste)
        # para que el template siga usando apikey_obj.key_plaintext sin cambios.
        instance.key_plaintext = services.get_api_key_plaintext(instance.pk)
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


def export_api_keys(request):
    """Export CSV (consolidado, abre en Excel) de las API keys filtradas.
    Respeta los filtros activos: q (busqueda), user (usuario), status (estado)."""
    if (redir := require_auth(request)):
        return redir
    csv_data = services.export_api_keys_csv(
        search=(request.GET.get("q") or "").strip(),
        user_id=(request.GET.get("user") or "") or None,
        status=(request.GET.get("status") or "") or None,
    )
    resp = HttpResponse(csv_data, content_type="text/csv; charset=utf-8")
    resp["Content-Disposition"] = 'attachment; filename="api-keys.csv"'
    return resp
