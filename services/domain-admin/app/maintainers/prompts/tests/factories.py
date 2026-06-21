"""Factories del mantenedor de Prompts.

Reusa core.tests.factories.make (pone un PK uuid si no se pasa `id`, ya que en
prod los genera domain-mcp). Solo agrega el helper específico de la tabla prompts.
"""
from __future__ import annotations

import uuid

from core.tests.factories import make

from maintainers.prompts.models import Prompt


def make_prompt(
    slug: str,
    *,
    project_id: uuid.UUID | str | None = None,
    version: int = 1,
    body: str = "Contenido del prompt.",
    description: str = "",
    is_active: bool = True,
    variables: list | None = None,
    tags: list[str] | None = None,
    created_by: uuid.UUID | str | None = None,
    deleted: bool = False,
) -> Prompt:
    p = make(
        Prompt,
        project_id=project_id,
        created_by=created_by,
        slug=slug,
        version=version,
        body=body,
        description=description,
        is_active=is_active,
        variables=variables if variables is not None else [],
        tags=tags if tags is not None else [],
    )
    if deleted:
        from django.utils import timezone
        p.deleted_at = timezone.now()
        p.is_active = False
        p.save()
    return p
