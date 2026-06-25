from __future__ import annotations

import uuid

from django.contrib import messages
from django.http import HttpResponse, HttpResponseRedirect
from django.shortcuts import render
from django.urls import reverse
from django.utils import timezone
from django.views.decorators.http import require_http_methods

from core.auth import require_auth
from core.views import MaintainerViews
from maintainers.apikeys.interfaces import ApiKeyServiceInterface
from maintainers.apikeys import services as apikeys_services

from . import services
from .forms import UserForm, UserRoleAssignForm
from .models import User


def _decrypt_api_keys(user_id, api_key_service: ApiKeyServiceInterface) -> list:
    from maintainers.apikeys.models import ApiKey
    keys = list(
        ApiKey.objects.filter(user_id=user_id, revoked_at__isnull=True)
        .exclude(status="revoked")
        .order_by("-created_at")
    )
    for k in keys:
        k.key_plaintext = api_key_service.get_api_key_plaintext(k.pk)
    return keys


class UserViews(MaintainerViews):
    def _form_payload(self, form) -> dict:
        return {
            "email": form.cleaned_data["email"],
            "name": form.composed_name(),
            "role_slug": form.cleaned_data["role"],
            "status": form.cleaned_data["status"],
            "hashed_password": form.hashed_password(),
        }

    def list(self, request):
        self._list_request = request
        return super().list(request)

    def do_list(self, search: str, page: int) -> dict:
        req = getattr(self, "_list_request", None)
        roles = req.GET.getlist("role") if req else []
        statuses = req.GET.getlist("status") if req else []
        return services.list_users(
            search=search, page=page, per_page=self.per_page,
            roles=roles, statuses=statuses,
        )

    def list_context(self, data: dict, search: str) -> dict:
        ctx = super().list_context(data, search)
        ctx["page_title"] = "Usuarios"
        req = getattr(self, "_list_request", None)

        ctx["role_options"] = services.list_role_options()
        ctx["status_options"] = User.STATUS_CHOICES
        ctx["selected_roles"] = req.GET.getlist("role") if req else []
        ctx["selected_statuses"] = req.GET.getlist("status") if req else []
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
            "api_keys": _decrypt_api_keys(instance.pk, apikeys_services),
        }


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
    per_page=10,
    search_param="q",
)


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
            messages.error(request, "Formulario invalido.")
    except services.UserError as exc:
        messages.error(request, str(exc))
    except Exception as exc:
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


@require_http_methods(["GET"])
def apikeys_modal(request):
    if (redir := require_auth(request)):
        return redir

    user_id = request.GET.get("user") or ""
    status = request.GET.get("status") or ""
    page = int(request.GET.get("page", 1) or 1)

    data = apikeys_services.list_api_keys(
        search="", page=page, per_page=10,
        user_id=user_id or None, status=status or None
    )
    return render(
        request,
        "users/_apikeys_modal.html",
        {
            "api_keys": data["api_keys"],
            "total": data["total"],
            "page": data["page"],
            "per_page": data["per_page"],
            "total_pages": data["total_pages"],
            "has_next": data["has_next"],
            "has_prev": data["has_prev"],
            "user_options": User.objects.filter(status="active").order_by("email"),
            "status_options": apikeys_services.ApiKeyService.model.STATUS_CHOICES,
            "selected_user": user_id,
            "selected_status": status,
        },
    )


def export_users(request):
    if (redir := require_auth(request)):
        return redir
    csv_data = services.export_users_csv(
        search=(request.GET.get("q") or "").strip(),
        roles=request.GET.getlist("role"),
        statuses=request.GET.getlist("status"),
    )
    resp = HttpResponse(csv_data, content_type="text/csv; charset=utf-8")
    resp["Content-Disposition"] = 'attachment; filename="usuarios.csv"'
    return resp


@require_http_methods(["GET"])
def invite_preview(request, user_id: str):
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
