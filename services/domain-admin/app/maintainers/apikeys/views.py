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
    def _form_payload(self, form) -> dict:
        return {
            "name": form.cleaned_data["name"],
            "expires_at": form.cleaned_data.get("expires_at"),
            "status": form.cleaned_data["status"],
        }

    def do_create(self, form):
        try:
            owner = User.objects.get(pk=form.cleaned_data["user"])
        except User.DoesNotExist as exc:
            raise services.ApiKeyError("El usuario seleccionado no existe.") from exc
        api_key, secret = services.create_api_key(
            user=owner,
            **self._form_payload(form),
        )

        messages.warning(
            self._request,
            f"Secreto de '{api_key.name}' (tambien visible en el detalle): {secret}",
        )
        return api_key

    def do_update(self, instance, form):
        return services.update_api_key(instance, **self._form_payload(form))

    def create(self, request):
        self._request = request
        return super().create(request)

    def edit(self, request, **kwargs):
        self._request = request
        return super().edit(request, **kwargs)

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

    def list_context(self, data: dict, search: str) -> dict:
        ctx = super().list_context(data, search)
        ctx["page_title"] = "API Keys"
        req = getattr(self, "_list_request", None)

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
        instance.key_plaintext = services.get_api_key_plaintext(instance.pk)
        return {"apikey_obj": instance, "object": instance}


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
    per_page=10,
    search_param="q",
)


def export_api_keys(request):
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
