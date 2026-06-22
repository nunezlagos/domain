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

from core.views import MaintainerViews

from . import services
from .forms import SkillForm
from .models import Skill


class SkillViews(MaintainerViews):
    """MaintainerViews especializado para skills (context keys propios)."""

    # core.do_list usa el MaintainerService generico sobre model.objects.all(),
    # que NO excluye los soft-deleted. skills SI debe excluirlos, asi que
    # delegamos en services.list_skills (que parte de un queryset filtrado por
    # deleted_at__isnull=True). Devuelve la lista bajo `skills` (list_key).
    def do_list(self, search: str, page: int) -> dict:
        return services.list_skills(search=search, page=page, per_page=self.per_page)

    def list_context(self, data: dict, search: str) -> dict:
        ctx = super().list_context(data, search)
        ctx["page_title"] = "Skills"
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


# Instancia que cablea todo. list_key="skills" -> el template recibe la lista
# bajo `skills`. id_kwarg="skill_id" -> casa con <uuid:skill_id> de las URLs.
# entity_label="Skill" -> core descubre get_skill/create_skill/... sin alias.
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
    per_page=20,
    search_param="q",
)
