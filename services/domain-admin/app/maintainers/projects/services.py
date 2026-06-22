"""Capa de negocio del mantenedor de Proyectos (migrada a core).

list + signal se delegan a core.service.MaintainerService (sin reimplementar la
busqueda/paginacion ni el aggregate de la señal). Lo propio del dominio queda aqui:

- list_projects filtra SOLO proyectos activos (deleted_at IS NULL) — por eso pasa
  un queryset pre-filtrado a MaintainerService.list en vez de usar el default.
- el estado activo/archivado vive en la columna `status` y en `deleted_at`
  (soft-delete), mantenidos en sync:
    * delete_project        -> soft-delete (deleted_at + status=archived).
    * toggle_project_status -> archiva (deleted_at=now + status=archived) o
      restaura (deleted_at=NULL + status=active).

Las views (core.views.MaintainerViews) descubren las funciones por convencion de
nombre. entity_label="Proyecto" -> attr "proyecto", por eso ademas exponemos
alias get_proyecto/... para el descubrimiento del core.

NOTA: la columna organization_id fue dropeada (tabla organizations eliminada);
ninguna query la referencia. El slug es unico globalmente.
"""
from __future__ import annotations

import re

from django.db import transaction
from django.utils import timezone

from core.service import MaintainerService

from .models import Project, ProjectRepository, ProjectTemplate


# Error de dominio (la view lo traduce a messages.error).
class ProjectError(Exception):
    """Error de operacion sobre proyectos."""


# Service base reusado: search (name/slug/description/repository_url) + signal.
class ProjectService(MaintainerService):
    model = Project
    search_fields = ("name", "slug", "description", "repository_url")
    ordering = ("-created_at",)


_service = ProjectService()


def list_projects(search: str = "", page: int = 1, per_page: int = 20) -> dict:
    """Lista proyectos ACTIVOS (no archivados) con busqueda + paginacion.

    Excluye los soft-deleted/archivados (deleted_at != NULL). Delega la
    busqueda/paginacion en MaintainerService.list pasando el queryset ya
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


# --- Skills enlazadas al proyecto (project_skills) --------------------------

def list_linked_skills(project: Project) -> list:
    """Skills enlazadas al proyecto (con flag link_enabled), ordenadas por slug."""
    from maintainers.projects.models import ProjectSkill
    from maintainers.skills.models import Skill

    links = list(ProjectSkill.objects.filter(project=project))
    enabled = {link.skill_id: link.is_enabled for link in links}
    skills = list(Skill.objects.filter(id__in=list(enabled.keys())).order_by("slug"))
    for s in skills:
        s.link_enabled = enabled.get(s.id, True)
    return skills


def list_available_skills(project: Project) -> list:
    """Skills de plataforma (globales) NO enlazadas todavia al proyecto."""
    from maintainers.projects.models import ProjectSkill
    from maintainers.skills.models import Skill

    linked_ids = list(
        ProjectSkill.objects.filter(project=project).values_list("skill_id", flat=True)
    )
    return list(
        Skill.objects.filter(project_id__isnull=True, deleted_at__isnull=True)
        .exclude(id__in=linked_ids)
        .order_by("slug")
    )


@transaction.atomic
def link_skill(project: Project, skill_id: str) -> None:
    """Enlaza una skill al proyecto (idempotente)."""
    from maintainers.projects.models import ProjectSkill

    ProjectSkill.objects.get_or_create(
        project=project, skill_id=skill_id, defaults={"is_enabled": True}
    )


@transaction.atomic
def unlink_skill(project: Project, skill_id: str) -> None:
    """Desenlaza una skill del proyecto."""
    from maintainers.projects.models import ProjectSkill

    ProjectSkill.objects.filter(project=project, skill_id=skill_id).delete()


# --- Repos git por proyecto -------------------------------------------------

def _derive_repo_name(url: str, index: int) -> str:
    """Alias del remoto derivado de la URL (ultimo segmento sin .git).

    Como ya no hay constraint UNIQUE(project_id, name) en la tabla, no es
    necesario garantizar unicidad; solo buscamos un alias legible. Fallback a
    'origin' para el primero y 'repo-N' para el resto.
    """
    base = url.rstrip("/").rsplit("/", 1)[-1]
    if base.endswith(".git"):
        base = base[:-4]
    base = re.sub(r"[^A-Za-z0-9._-]", "", base).strip("-")
    if base:
        return base[:50]
    return "origin" if index == 0 else f"repo-{index + 1}"


def _sync_repositories(project: Project, rows: list[dict]) -> None:
    """Reconcilia los remotos git del proyecto contra las filas del form.

    rows: lista de dicts {url, branch_default, root_path} (ya filtradas: url no
    vacia). Reconciliacion POR POSICION contra los repos activos existentes
    (orden estable por created_at): se actualizan en sitio los que calzan, se
    crean los extras y se soft-deletean los sobrantes. El primero queda como
    is_default. Ademas backfillea projects.repository_url con la URL del default
    (compat con el campo legacy de 1 repo principal).
    """
    existing = list(
        ProjectRepository.objects.filter(
            project=project, deleted_at__isnull=True
        ).order_by("created_at", "id")
    )

    for i, row in enumerate(rows):
        name = _derive_repo_name(row["url"], i)
        is_default = i == 0
        if i < len(existing):
            repo = existing[i]
            repo.name = name
            repo.url = row["url"]
            repo.branch_default = row.get("branch_default", "")
            repo.root_path = row.get("root_path", "")
            repo.is_default = is_default
            repo.save()
        else:
            ProjectRepository.objects.create(
                project=project,
                name=name,
                url=row["url"],
                branch_default=row.get("branch_default", ""),
                root_path=row.get("root_path", ""),
                is_default=is_default,
            )

    # Sobrantes: soft-delete (deleted_at) y quitarles el flag default.
    for repo in existing[len(rows):]:
        repo.deleted_at = timezone.now()
        repo.is_default = False
        repo.save()

    # Backfill de la URL principal legacy desde el repo default (primero).
    project.repository_url = rows[0]["url"] if rows else ""
    project.save(update_fields=["repository_url", "updated_at"])


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
    repositories: list[dict] | None = None,
) -> Project:
    """Crea un proyecto nuevo. slug unico global.

    `repositories` (si se pasa) es el set completo de remotos git como filas
    {url, branch_default, root_path}; se sincroniza y la URL principal se deriva
    del primero. Si es None, no se tocan repos (compat con callers legacy).
    """
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
    if repositories is not None:
        _sync_repositories(project, repositories)
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
    current_branch: str | None = None,
    client_id: str | None = None,
    repositories: list[dict] | None = None,
) -> Project:
    """Actualiza un proyecto. slug sigue siendo unico global.

    `repositories` (si se pasa) reemplaza el set de remotos git; la URL
    principal se re-deriva del primero. Si es None, no se tocan repos.
    `current_branch` es None cuando no se edita (el modal ya no lo expone): en
    ese caso se PRESERVA el valor existente (es referencial / de sistema).
    """
    if slug != project.slug and Project.objects.filter(
        slug=slug
    ).exclude(pk=project.pk).exists():
        raise ProjectError(f"Ya existe otro proyecto con slug '{slug}'.")

    if template_id and not ProjectTemplate.objects.filter(pk=template_id).exists():
        raise ProjectError(f"El template '{template_id}' no existe.")

    project.name = name
    project.slug = slug
    project.description = description or ""
    project.template_id = template_id or None
    if current_branch is not None:
        project.current_branch = current_branch
    project.client_id = client_id or None
    if repositories is None:
        # Sin gestion de repos: respetar la URL principal recibida (legacy).
        project.repository_url = repository_url or ""
    project.save()

    if repositories is not None:
        # _sync_repositories backfillea project.repository_url desde el default.
        _sync_repositories(project, repositories)
    return project


@transaction.atomic
def delete_project(project: Project) -> None:
    """Soft delete: marca deleted_at + status=archived. NO borra fisicamente."""
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


# --- Alias para el descubrimiento por convencion de core.views.MaintainerViews.
# entity_label="Proyecto" -> _entity_attr() == "proyecto", core busca
# get_proyecto / create_proyecto / update_proyecto / delete_proyecto /
# toggle_proyecto_status. Apuntamos esos nombres a las funciones reales.
get_proyecto = get_project
create_proyecto = create_project
update_proyecto = update_project
delete_proyecto = delete_project
toggle_proyecto_status = toggle_project_status
ServiceError = ProjectError
