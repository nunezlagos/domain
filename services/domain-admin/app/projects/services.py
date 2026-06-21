"""Capa de negocio del mantenedor de Proyectos.

Patrón: las views solo hacen HTTP request/response; toda la lógica de
modelo vive acá. Esto facilita testing unitario sin tocar HTTP.

Las tablas (projects / project_templates / project_repositories) las
administra domain-mcp (managed=False); Django solo lee/escribe vía ORM.

El estado activo/archivado se refleja en la columna `status` y en `deleted_at`
(soft-delete). Ambos se mantienen consistentes:
- delete_project  -> soft-delete (setea deleted_at + status=archived).
- toggle_project_status -> archiva (deleted_at=now + status=archived) o
  restaura (deleted_at=NULL + status=active). Es el "toggle" de estado.

NOTA: la columna organization_id fue dropeada (tabla organizations eliminada);
ninguna query la referencia. El slug es único globalmente.
"""
from __future__ import annotations

from django.db import transaction

from .models import Project, ProjectRepository, ProjectTemplate


# Error de dominio (la view lo traduce a messages.error).
class ProjectError(Exception):
    """Error de operación sobre proyectos."""


def list_projects(search: str = "", page: int = 1, per_page: int = 20) -> dict:
    """Lista proyectos ACTIVOS (no archivados) con búsqueda + paginación.

    Excluye los soft-deleted/archivados (deleted_at != NULL). Búsqueda sobre
    name / slug / description / repository_url.

    Retorna dict con: projects, total, page, per_page, total_pages,
    has_next, has_prev.
    """
    qs = Project.objects.filter(deleted_at__isnull=True)
    if search:
        qs = (
            qs.filter(name__icontains=search)
            | qs.filter(slug__icontains=search)
            | qs.filter(description__icontains=search)
            | qs.filter(repository_url__icontains=search)
        )
    qs = qs.distinct().order_by("-created_at")

    total = qs.count()
    total_pages = max(1, (total + per_page - 1) // per_page)
    start = (page - 1) * per_page
    end = start + per_page
    projects = list(qs[start:end])

    return {
        "projects": projects,
        "total": total,
        "page": page,
        "per_page": per_page,
        "total_pages": total_pages,
        "has_next": end < total,
        "has_prev": page > 1,
    }


def get_project(project_id: str) -> Project:
    try:
        return Project.objects.get(pk=project_id)
    except Project.DoesNotExist as exc:
        raise ProjectError(f"Proyecto {project_id} no existe.") from exc


def get_project_repositories(project: Project) -> list[ProjectRepository]:
    """Remotos git activos del proyecto (default primero)."""
    return list(
        ProjectRepository.objects.filter(project=project, deleted_at__isnull=True)
        .order_by("-is_default", "name")
    )


def list_available_templates() -> list[ProjectTemplate]:
    """Templates disponibles para el selector del form (ordenados por slug)."""
    return list(ProjectTemplate.objects.all().order_by("slug"))


@transaction.atomic
def create_project(
    *,
    name: str,
    slug: str,
    description: str = "",
    repository_url: str = "",
    template_id: str | None = None,
    current_branch: str = "",
    client_id: str | None = None,
) -> Project:
    """Crea un proyecto nuevo. slug único global."""
    if Project.objects.filter(slug=slug).exists():
        raise ProjectError(f"Ya existe un proyecto con slug '{slug}'.")

    if template_id and not ProjectTemplate.objects.filter(pk=template_id).exists():
        raise ProjectError(f"El template '{template_id}' no existe.")

    project = Project.objects.create(
        name=name,
        slug=slug,
        description=description or "",
        repository_url=repository_url or "",
        template_id=template_id or None,
        current_branch=current_branch or "",
        client_id=client_id or None,
    )
    return project


@transaction.atomic
def update_project(
    project: Project,
    *,
    name: str,
    slug: str,
    description: str = "",
    repository_url: str = "",
    template_id: str | None = None,
    current_branch: str = "",
    client_id: str | None = None,
) -> Project:
    """Actualiza un proyecto. slug sigue siendo único global."""
    if slug != project.slug and Project.objects.filter(
        slug=slug
    ).exclude(pk=project.pk).exists():
        raise ProjectError(f"Ya existe otro proyecto con slug '{slug}'.")

    if template_id and not ProjectTemplate.objects.filter(pk=template_id).exists():
        raise ProjectError(f"El template '{template_id}' no existe.")

    project.name = name
    project.slug = slug
    project.description = description or ""
    project.repository_url = repository_url or ""
    project.template_id = template_id or None
    project.current_branch = current_branch or ""
    project.client_id = client_id or None
    project.save()
    return project


@transaction.atomic
def delete_project(project: Project) -> None:
    """Soft delete: marca deleted_at + status=archived. NO borra físicamente."""
    from django.utils import timezone

    project.deleted_at = timezone.now()
    project.status = Project.STATUS_ARCHIVED
    project.save()


@transaction.atomic
def toggle_project_status(project: Project) -> str:
    """Alterna activo <-> archivado. Mantiene deleted_at y status en sync.

    - Proyecto activo (deleted_at IS NULL) -> archivado.
    - Proyecto archivado -> restaurado.
    """
    from django.utils import timezone

    if project.deleted_at is None:
        project.deleted_at = timezone.now()
        project.status = Project.STATUS_ARCHIVED
    else:
        project.deleted_at = None
        project.status = Project.STATUS_ACTIVE
    project.save()
    return project.status


def get_list_signal() -> dict:
    """Señal barata de cambios para refresh on-change.

    NO es polling ciego de la tabla. Devuelve count + max(updated_at):
    cualquier alta, edición, baja (soft) o toggle muta uno de los dos
    (updated_at lo bumpea el trigger set_updated_at en la BD; created_at de
    altas nuevas sube el max). El front compara contra su última señal y solo
    re-renderiza la tabla cuando algo cambió en la BD — incluyendo inserts de
    otros servicios (domain-mcp) que escriben directo en `projects`.

    Query barata: SELECT count(*), max(updated_at) FROM projects.
    """
    from django.db.models import Count, Max

    agg = Project.objects.aggregate(total=Count("id"), latest=Max("updated_at"))
    latest = agg["latest"]
    return {
        "count": agg["total"] or 0,
        "version": latest.isoformat() if latest else "",
    }


def get_stats() -> dict:
    """Stats agregadas para el header del listado."""
    active = Project.objects.filter(deleted_at__isnull=True).count()
    archived = Project.objects.filter(deleted_at__isnull=False).count()
    return {
        "total": active + archived,
        "active": active,
        "archived": archived,
    }
