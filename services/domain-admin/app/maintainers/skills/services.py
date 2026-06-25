"""Capa de negocio del mantenedor de Skills (migrada a core).

list + signal se delegan a core.service.MaintainerService (sin reimplementar la
busqueda/paginacion ni el aggregate de la señal). El resto —scope de slug por
(project_id, slug), parseo de tags, create/update/delete con sus validaciones de
dominio, y la lista READ-ONLY de versiones— sigue aqui.

Las views (core.views.MaintainerViews) descubren las funciones por convencion
de nombre: entity_label="Skill" -> attr "skill", asi que core busca get_skill /
create_skill / update_skill / delete_skill / get_list_signal, que ya coinciden
con los nombres reales (no hacen falta alias como en users).

skills SI tiene `deleted_at` -> soft-delete. La unicidad de slug es por scope:
(project_id, slug), donde project_id NULL = skill global. skill_versions es
read-only (sin CRUD).
"""
from __future__ import annotations

from django.db import transaction

from core.service import MaintainerService

from .models import Skill, SkillVersion



class SkillError(Exception):
    """Error de operacion sobre skills."""




class SkillService(MaintainerService):
    model = Skill
    search_fields = ("name", "slug", "description")
    ordering = ("-created_at",)

    def base_qs(self):
        return Skill.objects.filter(deleted_at__isnull=True)


_service = SkillService()


def _slug_taken(slug: str, project_id, exclude_pk=None) -> bool:
    """True si ya existe una skill viva con ese slug en el mismo scope.

    El scope es el project_id (NULL = global). Excluye soft-deleted y,
    opcionalmente, el propio registro (edicion).
    """
    qs = Skill.objects.filter(deleted_at__isnull=True, slug=slug)
    if project_id in (None, ""):
        qs = qs.filter(project_id__isnull=True)
    else:
        qs = qs.filter(project_id=project_id)
    if exclude_pk is not None:
        qs = qs.exclude(pk=exclude_pk)
    return qs.exists()


def _filtered_skill_qs(skill_types=None):
    """Queryset de Skill filtrado por tipo (multi-select). Lista vacia = sin
    filtro. Parte del queryset base (excluye soft-deleted) y se pasa a
    MaintainerService.list (que suma la busqueda)."""
    qs = _service.base_qs()
    if skill_types:
        qs = qs.filter(skill_type__in=skill_types)
    return qs


def list_skills(search: str = "", page: int = 1, per_page: int = 20,
                skill_types=None) -> dict:
    """Lista skills (excluye soft-deleted) con busqueda + filtro de tipo +
    paginacion.

    Delega en MaintainerService.list pasando el queryset ya filtrado y renombra
    la clave `items` -> `skills` para no romper el template/tests.
    """
    data = _service.list(
        qs=_filtered_skill_qs(skill_types), search=search, page=page,
        per_page=per_page,
    )
    data["skills"] = data.pop("items")
    return data


def export_skills_csv(search: str = "", skill_types=None) -> str:
    """CSV consolidado (compatible con Excel) de las skills que matchean los
    filtros activos (tipo/busqueda). Excluye soft-deleted. Sin paginar."""
    import csv
    import io

    from django.db.models import Q

    qs = _filtered_skill_qs(skill_types)
    if search:
        qs = qs.filter(
            Q(name__icontains=search)
            | Q(slug__icontains=search)
            | Q(description__icontains=search)
        )
    qs = qs.distinct().order_by("name")

    buf = io.StringIO()
    w = csv.writer(buf)
    w.writerow(["Nombre", "Slug", "Tipo", "Descripcion", "Creado"])
    for s in qs:
        w.writerow([
            s.name, s.slug, s.get_skill_type_display(), s.description,
            s.created_at.strftime("%Y-%m-%d %H:%M") if s.created_at else "",
        ])
    return buf.getvalue()


def get_list_signal() -> dict:
    """Señal barata de cambios {count, version} para refresh on-change."""
    return _service.list_signal()


def get_skill(skill_id: str) -> Skill:
    try:
        return Skill.objects.get(pk=skill_id)
    except Skill.DoesNotExist as exc:
        raise SkillError(f"Skill {skill_id} no existe.") from exc


def get_skill_versions(skill: Skill) -> list[SkillVersion]:
    """Versiones (snapshots) de una skill, mas nueva primero. Read-only."""
    return list(SkillVersion.objects.filter(skill=skill).order_by("-version"))


@transaction.atomic
def create_skill(
    *,
    slug: str,
    name: str,
    skill_type: str = "prompt",
    description: str = "",
    content: str = "",
    timeout_seconds: int = 30,
    idempotent: bool = False,
    has_side_effects: bool = False,
    tags: list[str] | None = None,
    project_id=None,
) -> Skill:
    """Crea una skill nueva. slug debe ser unico dentro de su scope."""
    if _slug_taken(slug, project_id):
        raise SkillError(f"Ya existe una skill con slug '{slug}' en este scope.")

    return Skill.objects.create(
        slug=slug,
        name=name,
        skill_type=skill_type,
        description=description or "",
        content=content or "",
        timeout_seconds=timeout_seconds,
        idempotent=idempotent,
        has_side_effects=has_side_effects,
        tags=tags or [],
        project_id=project_id or None,
    )


@transaction.atomic
def update_skill(
    skill: Skill,
    *,
    slug: str,
    name: str,
    skill_type: str = "prompt",
    description: str = "",
    content: str = "",
    timeout_seconds: int = 30,
    idempotent: bool = False,
    has_side_effects: bool = False,
    tags: list[str] | None = None,
) -> Skill:
    """Actualiza una skill. El slug sigue siendo unico dentro de su scope.

    El scope (project_id) no se cambia desde el admin; se valida contra el
    project_id actual del registro, excluyendose a si mismo.
    """
    if slug != skill.slug and _slug_taken(slug, skill.project_id, exclude_pk=skill.pk):
        raise SkillError(f"Ya existe otra skill con slug '{slug}' en este scope.")

    skill.slug = slug
    skill.name = name
    skill.skill_type = skill_type
    skill.description = description or ""
    skill.content = content or ""
    skill.timeout_seconds = timeout_seconds
    skill.idempotent = idempotent
    skill.has_side_effects = has_side_effects
    skill.tags = tags or []
    skill.save()
    return skill


@transaction.atomic
def delete_skill(skill: Skill) -> None:
    """Soft delete: marca deleted_at. NO borra fisicamente."""
    from django.utils import timezone

    skill.deleted_at = timezone.now()
    skill.save()


def get_stats() -> dict:
    """Stats agregadas para el header del listado."""
    base = Skill.objects.filter(deleted_at__isnull=True)
    return {
        "total": base.count(),
        "prompt": base.filter(skill_type="prompt").count(),
        "proposed": base.filter(proposed=True).count(),
    }



ServiceError = SkillError
