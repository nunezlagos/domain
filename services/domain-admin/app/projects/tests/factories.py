"""Helpers para crear filas reales en la DB de test.

Los PKs son UUID; en prod los genera domain-mcp, así que en tests los pasamos
explícitamente. organization_id es obligatorio en `projects` /
`project_repositories`, por eso cada factory tiene un default.
"""
from __future__ import annotations

import uuid

from projects.models import Project, ProjectRepository, ProjectTemplate

# org fija reutilizable: simula la organización "actual" del admin.
DEFAULT_ORG = uuid.UUID("11111111-1111-1111-1111-111111111111")


def make_template(slug: str = "base", *, name: str | None = None,
                  organization_id: uuid.UUID | None = DEFAULT_ORG,
                  is_public: bool = False, is_default: bool = False) -> ProjectTemplate:
    return ProjectTemplate.objects.create(
        id=uuid.uuid4(),
        organization_id=organization_id,
        slug=slug,
        name=name or slug.capitalize(),
        is_public=is_public,
        is_default=is_default,
    )


def make_project(name: str = "Demo", *, slug: str | None = None,
                 organization_id: uuid.UUID = DEFAULT_ORG,
                 description: str = "", repository_url: str = "",
                 template_id: uuid.UUID | None = None,
                 current_branch: str = "", archived: bool = False) -> Project:
    p = Project.objects.create(
        id=uuid.uuid4(),
        organization_id=organization_id,
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
        p.save()
    return p


def make_repository(project: Project, name: str = "origin", *,
                    url: str = "https://github.com/org/repo",
                    is_default: bool = False, kind: str = "github",
                    deleted: bool = False) -> ProjectRepository:
    r = ProjectRepository.objects.create(
        id=uuid.uuid4(),
        organization_id=project.organization_id,
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
