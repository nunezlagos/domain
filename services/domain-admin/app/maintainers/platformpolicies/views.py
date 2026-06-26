"""Views del mantenedor de Politicas de plataforma.

Las 7 vistas estandar las arma core.views.MaintainerViews. Aqui solo el payload
del service y los context keys (policy_obj).
"""
from __future__ import annotations

from core.views import MaintainerViews

from . import services
from .forms import PlatformPolicyForm
from .models import PlatformPolicy


class PlatformPolicyViews(MaintainerViews):
    def do_create(self, form):
        cd = form.cleaned_data
        return services.create_policy(
            slug=cd["slug"],
            name=cd["name"],
            kind=cd["kind"],
            body_md=cd["body_md"],
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
            is_active=cd["is_active"],
        )

    def list_context(self, data: dict, search: str) -> dict:
        ctx = super().list_context(data, search)
        ctx["page_title"] = "Politicas de plataforma"
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


views = PlatformPolicyViews(
    app_name="platformpolicies",
    model=PlatformPolicy,
    form_class=PlatformPolicyForm,
    service=services,
    templates="platformpolicies",
    search_fields=("name", "slug", "body_md"),
    entity_label="Politica",
    id_kwarg="policy_id",
    list_key="policies",
    per_page=10,
    search_param="q",
)
