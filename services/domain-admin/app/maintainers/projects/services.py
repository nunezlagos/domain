"""Capa de negocio del mantenedor de Proyectos (migrada a core).

list + signal se delegan a core.service.MaintainerService (sin reimplementar la
búsqueda/paginación ni el aggregate de la señal). Lo propio del dominio queda acá:

- list_projects filtra SOLO proyectos activos (deleted_at IS NULL) — por eso pasa
  un queryset pre-filtrado a MaintainerService.list en vez de usar el default.
- el estado activo/archivado vive en la columna `status` y en `deleted_at`
  (soft-delete), mantenidos en sync:
    * delete_project        -> soft-delete (deleted_at + status=archived).
    * toggle_project_status -> archiva (deleted_at=now + status=archived) o
      restaura (deleted_at=NULL + status=active).

Las views (core.views.MaintainerViews) descubren las funciones por convención de
nombre. entity_label="Proyecto" -> attr "proyecto", por eso además exponemos
alias get_proyecto/... para el descubrimiento del core.

NOTA: la columna organization_id fue dropeada (tabla organizations eliminada);
ninguna query la referencia. El slug es único globalmente.
"""
from __future__ import annotations

from django.db import transaction

from core.service import MaintainerService

from .models import Project, ProjectRepository, ProjectTemplate


# Error de dominio (la view lo traduce a messages.error).
class ProjectError(Exception):
    """Error de operación sobre proyectos."""


# Service base reusado: search (name/slug/description/repository_url) + signal.
class ProjectService(MaintainerService):
    model = Project
    search_fields = ("name", "slug", "description", "repository_url")
    ordering = ("-created_at",)


_service = ProjectService()


def list_projects(search: str = "", page: int = 1, per_page: int = 20) -> dict:
    """Lista proyectos ACTIVOS (no archivados) con búsqueda + paginación.

    Excluye los soft-deleted/archivados (deleted_at != NULL). Delega la
    búsqueda/paginación en MaintainerService.list pasando el queryset ya
    filtrado, y renombra la clave `items` -> `projects` para no romper el
    contrato del template/tests existentes.
    """
    qs = Project.objects.filter(deleted_at__isnull=True)
    data = _service.list(qs=qs, search=search, page=page, per_page=per_page)
    data["projects"] = data.pop("items")
    return data


def get_list_signal() -> dict:
    """Señal barata de cambios {count, version} para refresh on-change."""
    return _service.list_signal()


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


def get_stats() -> dict:
    """Stats agregadas para el header del listado."""
    active = Project.objects.filter(deleted_at__isnull=True).count()
    archived = Project.objects.filter(deleted_at__isnull=False).count()
    return {
        "total": active + archived,
        "active": active,
        "archived": archived,
    }


# --- Alias para el descubrimiento por convención de core.views.MaintainerViews.
# entity_label="Proyecto" -> _entity_attr() == "proyecto", core busca
# get_proyecto / create_proyecto / update_proyecto / delete_proyecto /
# toggle_proyecto_status. Apuntamos esos nombres a las funciones reales.
get_proyecto = get_project
create_proyecto = create_project
update_proyecto = update_project
delete_proyecto = delete_project
toggle_proyecto_status = toggle_project_status
ServiceError = ProjectError
