"""Helpers para crear filas reales en la DB de test.

Los PKs son UUID (en prod los genera domain-mcp), así que en tests hay que
pasarlos explícitamente. La unicidad de slug es por (project_id, slug);
DEFAULT_PROJECT permite forzar choques de slug dentro del mismo scope, y
project_id=None representa el scope global.
"""
from __future__ import annotations

import uuid

from skills.models import Skill, SkillVersion

# Proyecto por defecto para forzar choques de slug dentro de un mismo scope
# no-global (la unicidad real es por (project_id, slug)).
DEFAULT_PROJECT = uuid.UUID("22222222-2222-2222-2222-222222222222")


def make_skill(
    name: str,
    *,
    slug: str | None = None,
    skill_type: str = "prompt",
    description: str = "",
    content: str = "",
    timeout_seconds: int = 30,
    idempotent: bool = False,
    has_side_effects: bool = False,
    tags: list[str] | None = None,
    project_id: uuid.UUID | str | None = None,
    proposed: bool = False,
    deleted: bool = False,
) -> Skill:
    if slug is None:
        slug = name.lower().replace(" ", "-")
    s = Skill.objects.create(
        id=uuid.uuid4(),
        slug=slug,
        name=name,
        skill_type=skill_type,
        description=description,
        content=content,
        timeout_seconds=timeout_seconds,
        idempotent=idempotent,
        has_side_effects=has_side_effects,
        tags=tags or [],
        project_id=project_id,
        proposed=proposed,
    )
    if deleted:
        from django.utils import timezone
        s.deleted_at = timezone.now()
        s.save()
    return s


def make_skill_version(
    skill: Skill,
    *,
    version: int,
    changelog: str = "",
    content: str = "",
    created_by: uuid.UUID | str | None = None,
) -> SkillVersion:
    return SkillVersion.objects.create(
        id=uuid.uuid4(),
        skill=skill,
        version=version,
        changelog=changelog,
        content=content,
        created_by=created_by,
    )
