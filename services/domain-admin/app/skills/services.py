"""Capa de negocio del mantenedor de Skills.

Patrón: las views solo hacen HTTP request/response; toda la lógica de modelo
vive acá. Esto facilita testing unitario sin tocar HTTP.

La tabla `skills` la administra domain-mcp (managed=False); Django solo
lee/escribe vía ORM.

skills NO tiene columna `status` (no hay toggle). SÍ tiene `deleted_at`
→ soft-delete. La unicidad de slug es por scope: (project_id, slug), donde
project_id NULL = skill global. skill_versions es read-only (sin CRUD).
"""
from __future__ import annotations

from django.db import transaction

from .models import Skill, SkillVersion


# Error de dominio (la view lo traduce a messages.error).
class SkillError(Exception):
    """Error de operación sobre skills."""


def _slug_taken(slug: str, project_id, exclude_pk=None) -> bool:
    """True si ya existe una skill viva con ese slug en el mismo scope.

    El scope es el project_id (NULL = global). Excluye soft-deleted y,
    opcionalmente, el propio registro (edición).
    """
    qs = Skill.objects.filter(deleted_at__isnull=True, slug=slug)
    if project_id in (None, ""):
        qs = qs.filter(project_id__isnull=True)
    else:
        qs = qs.filter(project_id=project_id)
    if exclude_pk is not None:
        qs = qs.exclude(pk=exclude_pk)
    return qs.exists()


def list_skills(search: str = "", page: int = 1, per_page: int = 20) -> dict:
    """Lista skills con búsqueda opcional + paginación.

    Excluye los soft-deleted (deleted_at != NULL). Búsqueda sobre
    name / slug / description.

    Retorna dict con: skills, total, page, per_page, total_pages,
    has_next, has_prev.
    """
    qs = Skill.objects.filter(deleted_at__isnull=True)
    if search:
        qs = (
            qs.filter(name__icontains=search)
            | qs.filter(slug__icontains=search)
            | qs.filter(description__icontains=search)
        )
    qs = qs.distinct().order_by("-created_at")

    total = qs.count()
    total_pages = max(1, (total + per_page - 1) // per_page)
    start = (page - 1) * per_page
    end = start + per_page
    skills = list(qs[start:end])

    return {
        "skills": skills,
        "total": total,
        "page": page,
        "per_page": per_page,
        "total_pages": total_pages,
        "has_next": end < total,
        "has_prev": page > 1,
    }


def get_skill(skill_id: str) -> Skill:
    try:
        return Skill.objects.get(pk=skill_id)
    except Skill.DoesNotExist as exc:
        raise SkillError(f"Skill {skill_id} no existe.") from exc


def get_skill_versions(skill: Skill) -> list[SkillVersion]:
    """Versiones (snapshots) de una skill, más nueva primero. Read-only."""
    return list(
        SkillVersion.objects.filter(skill=skill).order_by("-version")
    )


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
    """Crea una skill nueva. slug debe ser único dentro de su scope."""
    if _slug_taken(slug, project_id):
        raise SkillError(
            f"Ya existe una skill con slug '{slug}' en este scope."
        )

    skill = Skill.objects.create(
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
    return skill


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
    """Actualiza una skill. El slug sigue siendo único dentro de su scope.

    El scope (project_id) no se cambia desde el admin; se valida contra el
    project_id actual del registro, excluyéndose a sí mismo.
    """
    if slug != skill.slug and _slug_taken(slug, skill.project_id, exclude_pk=skill.pk):
        raise SkillError(
            f"Ya existe otra skill con slug '{slug}' en este scope."
        )

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
    """Soft delete: marca deleted_at. NO borra físicamente.

    skills no tiene columna `status`, así que solo se setea deleted_at
    (que es lo que filtra list_skills y los índices parciales reales).
    """
    from django.utils import timezone

    skill.deleted_at = timezone.now()
    skill.save()


def get_list_signal() -> dict:
    """Señal barata de cambios para refresh on-change.

    NO es polling ciego. Devuelve count + max(updated_at): cualquier alta,
    edición o baja (soft) muta uno de los dos (updated_at lo bumpea el
    trigger set_updated_at en la BD; created_at de altas nuevas sube el max).
    El front compara contra su última señal y solo re-renderiza la tabla
    cuando algo cambió en la BD — incluyendo inserts de domain-mcp que
    escriben directo en `skills`.
    """
    from django.db.models import Count, Max

    agg = Skill.objects.aggregate(total=Count("id"), latest=Max("updated_at"))
    latest = agg["latest"]
    return {
        "count": agg["total"] or 0,
        "version": latest.isoformat() if latest else "",
    }


def get_stats() -> dict:
    """Stats agregadas para el header del listado."""
    base = Skill.objects.filter(deleted_at__isnull=True)
    return {
        "total": base.count(),
        "prompt": base.filter(skill_type="prompt").count(),
        "proposed": base.filter(proposed=True).count(),
    }
