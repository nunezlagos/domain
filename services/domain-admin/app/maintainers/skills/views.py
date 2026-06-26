"""Views del mantenedor de Skills (migradas a core).

Las 7 vistas estandar (list, signal, detail, create, edit, delete, toggle) las
arma core.views.MaintainerViews. Aqui solo:

  1. Se configura la instancia `views` (model/form/service/templates/labels).
  2. Se sobreescriben los context builders especificos de skills: los templates
     usan `skill_obj` (no `object`) y el detail expone `skill_versions`
     (lista READ-ONLY de snapshots).

El payload del service calza 1:1 con el cleaned_data del form (slug, name,
skill_type, description, content, timeout_seconds, idempotent, has_side_effects,
tags), asi que NO hace falta sobreescribir _form_payload.

skills NO tiene toggle de estado en la UI (la baja es soft-delete via
deleted_at); el boton toggle simplemente no se renderiza en los templates.

El guard de auth (require_auth) y la deteccion AJAX (is_ajax) vienen de
core.auth (antes estaban duplicados como _require_auth/_is_ajax).
"""
from __future__ import annotations

from django.http import HttpResponse
from django.shortcuts import get_object_or_404, redirect

from core.auth import require_auth
from core.views import MaintainerViews

from . import services
from .forms import SkillForm
from .models import Skill


class SkillViews(MaintainerViews):
    """MaintainerViews especializado para skills (context keys propios)."""



    def list(self, request):
        self._list_request = request
        return super().list(request)





    def do_list(self, search: str, page: int) -> dict:
        req = getattr(self, "_list_request", None)
        skill_type = req.GET.get("skill_type") if req else ""
        return services.list_skills(
            search=search, page=page, per_page=self.per_page,
            skill_types=[skill_type] if skill_type else None,
        )

    def list_context(self, data: dict, search: str) -> dict:
        ctx = super().list_context(data, search)
        ctx["page_title"] = "Skills"
        req = getattr(self, "_list_request", None)

        ctx["skill_type_options"] = Skill.SKILL_TYPE_CHOICES
        ctx["selected_skill_type"] = req.GET.get("skill_type") if req else ""
        return ctx

    def form_context(self, form, mode: str, instance, action: str) -> dict:
        return {
            "form": form,
            "mode": mode,
            "skill_obj": instance,
            "object": instance,
            "action": action,
        }

    def detail_context(self, instance) -> dict:
        return {
            "skill_obj": instance,
            "object": instance,
            "skill_versions": services.get_skill_versions(instance),
        }

    def do_create(self, form):
        # Mapea el ChoiceField "project" -> project_id y pasa root_path; el
        # payload genérico no sirve porque create_skill espera project_id.
        cd = form.cleaned_data
        return services.create_skill(
            slug=cd["slug"], name=cd["name"], skill_type=cd["skill_type"],
            description=cd.get("description", ""), content=cd.get("content", ""),
            timeout_seconds=cd["timeout_seconds"],
            idempotent=cd.get("idempotent", False),
            has_side_effects=cd.get("has_side_effects", False),
            tags=cd.get("tags", []),
            project_id=(cd.get("project") or None),
            root_path=(cd.get("root_path") or None),
        )

    def do_update(self, instance, form):
        cd = form.cleaned_data
        return services.update_skill(
            instance, slug=cd["slug"], name=cd["name"], skill_type=cd["skill_type"],
            description=cd.get("description", ""), content=cd.get("content", ""),
            timeout_seconds=cd["timeout_seconds"],
            idempotent=cd.get("idempotent", False),
            has_side_effects=cd.get("has_side_effects", False),
            tags=cd.get("tags", []),
            root_path=(cd.get("root_path") or None),
        )





views = SkillViews(
    app_name="skills",
    model=Skill,
    form_class=SkillForm,
    service=services,
    templates="skills",
    search_fields=("name", "slug", "description"),
    entity_label="Skill",
    id_kwarg="skill_id",
    list_key="skills",
    per_page=10,
    search_param="q",
)


def export_skills(request):
    """Export CSV (consolidado, abre en Excel) de las skills filtradas.
    Respeta los filtros activos: q (busqueda) + skill_type (tipo)."""
    if (redir := require_auth(request)):
        return redir
    skill_type = (request.GET.get("skill_type") or "").strip()
    csv_data = services.export_skills_csv(
        search=(request.GET.get("q") or "").strip(),
        skill_types=[skill_type] if skill_type else None,
    )
    resp = HttpResponse(csv_data, content_type="text/csv; charset=utf-8")
    resp["Content-Disposition"] = 'attachment; filename="skills.csv"'
    return resp


def approve_skill(request, skill_id):
    """Aprueba una skill propuesta (proposed=true -> false). POST."""
    if (redir := require_auth(request)):
        return redir
    skill = get_object_or_404(Skill, pk=skill_id, deleted_at__isnull=True)
    services.approve_skill(skill)
    return redirect("skills:detail", skill_id=skill.pk)


def reject_skill(request, skill_id):
    """Rechaza una skill propuesta (soft-delete). POST."""
    if (redir := require_auth(request)):
        return redir
    skill = get_object_or_404(Skill, pk=skill_id, deleted_at__isnull=True)
    services.reject_skill(skill)
    return redirect("skills:list")
