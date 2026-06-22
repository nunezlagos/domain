"""Factories del mantenedor de Proyectos.

Reusa core.tests.factories.make (pone un PK uuid si no se pasa `id`, ya que en
prod los genera domain-mcp). Solo agrega los helpers especificos de las 3 tablas
del dominio (projects, project_templates, project_repositories). La columna
organization_id fue dropeada del schema real, por eso ningun factory la setea.
"""
from __future__ import annotations

import uuid

from core.tests.factories import make

from maintainers.projects.models import Project, ProjectRepository, ProjectTemplate


def make_template(slug: str = "base", *, name: str | None = None,
                  is_public: bool = False, is_default: bool = False) -> ProjectTemplate:
    return make(
        ProjectTemplate,
        slug=slug,
        name=name or slug.capitalize(),
        is_public=is_public,
        is_default=is_default,
    )


def make_project(name: str = "Demo", *, slug: str | None = None,
                 description: str = "", repository_url: str = "",
                 template_id: uuid.UUID | None = None,
                 current_branch: str = "", archived: bool = False) -> Project:
    p = make(
        Project,
        name=name,
        slug=slug or name.lower().replace(" ", "-"),
        description=description,
        repository_url=repository_url,
        template_id=template_id,
        current_branch=current_branch,
    )
    if archived:
        from django.utils import timezone
        p.deleted_at = timezone.now()
        p.status = Project.STATUS_ARCHIVED
        p.save()
    return p


def make_repository(project: Project, name: str = "origin", *,
                    url: str = "https://github.com/org/repo",
                    is_default: bool = False, kind: str = "github",
                    deleted: bool = False) -> ProjectRepository:
    r = make(
        ProjectRepository,
        project=project,
        name=name,
        url=url,
        is_default=is_default,
        kind=kind,
    )
    if deleted:
        from django.utils import timezone
        r.deleted_at = timezone.now()
        r.save()
    return r
