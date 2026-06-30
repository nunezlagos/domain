from __future__ import annotations

from django.db import transaction
from django.utils import timezone

from core.service import MaintainerService

from .models import Project, ProjectTemplate


class ProjectError(Exception):
    pass


class ProjectService(MaintainerService):
    model = Project
    search_fields = ("name", "slug", "description", "repository_url")
    ordering = ("-created_at",)


_service = ProjectService()


def _filtered_project_qs(statuses=None):
    qs = Project.objects.all()
    if statuses:
        qs = qs.filter(status__in=statuses)
    return qs


def list_projects(search: str = "", page: int = 1, per_page: int = 20,
                  statuses=None) -> dict:
    qs = _filtered_project_qs(statuses).filter(deleted_at__isnull=True)
    data = _service.list(qs=qs, search=search, page=page, per_page=per_page)
    data["projects"] = data.pop("items")
    return data


def export_projects_csv(search: str = "", statuses=None) -> str:
    import csv
    import io
    from django.db.models import Q

    qs = _filtered_project_qs(statuses)
    if search:
        qs = qs.filter(
            Q(name__icontains=search)
            | Q(slug__icontains=search)
            | Q(description__icontains=search)
            | Q(repository_url__icontains=search)
        )
    qs = qs.distinct().order_by("name")

    buf = io.StringIO()
    w = csv.writer(buf)
    w.writerow(["Nombre", "Slug", "Descripcion", "Estado", "Creado"])
    for p in qs:
        w.writerow([
            p.name, p.slug, p.description, p.get_status_display(),
            p.created_at.strftime("%Y-%m-%d %H:%M") if p.created_at else "",
        ])
    return buf.getvalue()


def get_list_signal() -> dict:
    return _service.list_signal()


def get_project(project_id: str) -> Project:
    try:
        return Project.objects.get(pk=project_id)
    except Project.DoesNotExist as exc:
        raise ProjectError(f"Proyecto {project_id} no existe.") from exc


def list_available_templates() -> list[ProjectTemplate]:
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
    repositories: list[dict] | None = None,
) -> Project:
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
    current_branch: str | None = None,
    client_id: str | None = None,
    repositories: list[dict] | None = None,
) -> Project:
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
        project.repository_url = repository_url or ""
    project.save()

    return project


@transaction.atomic
def delete_project(project: Project) -> None:
    project.deleted_at = timezone.now()
    project.status = Project.STATUS_ARCHIVED
    project.save()


@transaction.atomic
def toggle_project_status(project: Project) -> str:
    if project.deleted_at is None:
        project.deleted_at = timezone.now()
        project.status = Project.STATUS_ARCHIVED
    else:
        project.deleted_at = None
        project.status = Project.STATUS_ACTIVE
    project.save()
    return project.status


def get_stats() -> dict:
    active = Project.objects.filter(deleted_at__isnull=True).count()
    archived = Project.objects.filter(deleted_at__isnull=False).count()
    return {
        "total": active + archived,
        "active": active,
        "archived": archived,
    }


get_proyecto = get_project
create_proyecto = create_project
update_proyecto = update_project
delete_proyecto = delete_project
toggle_proyecto_status = toggle_project_status
ServiceError = ProjectError
