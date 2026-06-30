"""Views del mantenedor de Reglas por proyecto.

Las 7 vistas estandar las arma core.views.MaintainerViews. Aqui solo el payload
del service (mapea el `project` ChoiceField -> project_id en alta; en edicion el
proyecto no cambia) y los context keys (`policy_obj`).
"""
from __future__ import annotations

from django.shortcuts import get_object_or_404, redirect

from core.auth import require_auth
from core.views import MaintainerViews

from . import services
from .forms import ProjectPolicyForm
from .models import ProjectPolicy


class ProjectPolicyViews(MaintainerViews):
    def do_create(self, form):
        cd = form.cleaned_data
        return services.create_policy(
            project_id=cd.get("project"),
            slug=cd["slug"],
            name=cd["name"],
            kind=cd["kind"],
            body_md=cd["body_md"],
            override_platform=cd["override_platform"],
            is_active=cd["is_active"],
        )

    def do_update(self, instance, form):
        cd = form.cleaned_data
        return services.update_policy(
            instance,
            slug=cd["slug"],
            name=cd["name"],
            kind=cd["kind"],
            body_md=cd["body_md"],
            override_platform=cd["override_platform"],
            is_active=cd["is_active"],
        )

    def list_context(self, data: dict, search: str) -> dict:
        ctx = super().list_context(data, search)
        ctx["page_title"] = "Reglas por proyecto"
        return ctx

    def form_context(self, form, mode: str, instance, action: str) -> dict:
        return {
            "form": form,
            "mode": mode,
            "policy_obj": instance,
            "object": instance,
            "action": action,
        }

    def detail_context(self, instance) -> dict:
        return {"policy_obj": instance, "object": instance}


views = ProjectPolicyViews(
    app_name="projectpolicies",
    model=ProjectPolicy,
    form_class=ProjectPolicyForm,
    service=services,
    templates="projectpolicies",
    search_fields=("name", "slug", "body_md"),
    entity_label="Regla",
    id_kwarg="policy_id",
    list_key="policies",
    per_page=10,
    search_param="q",
)


def approve_policy(request, policy_id):
    """Aprueba una policy propuesta (proposed=true -> false). POST."""
    if (redir := require_auth(request)):
        return redir
    policy = get_object_or_404(ProjectPolicy, pk=policy_id, deleted_at__isnull=True)
    services.approve_policy(policy)
    return redirect("projectpolicies:detail", policy_id=policy.pk)


def reject_policy(request, policy_id):
    """Rechaza una policy propuesta (soft-delete). POST."""
    if (redir := require_auth(request)):
        return redir
    policy = get_object_or_404(ProjectPolicy, pk=policy_id, deleted_at__isnull=True)
    services.reject_policy(policy)
    return redirect("projectpolicies:list")
