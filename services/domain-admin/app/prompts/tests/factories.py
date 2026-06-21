"""Helpers para crear filas reales en la DB de test.

Los PKs son UUID (en prod los genera domain-mcp), así que en tests hay
que pasarlos explícitamente.
"""
from __future__ import annotations

import uuid

from prompts.models import Prompt


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
    p = Prompt.objects.create(
        id=uuid.uuid4(),
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
