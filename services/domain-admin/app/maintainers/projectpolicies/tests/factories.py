"""Factories del mantenedor de Reglas por proyecto."""
from __future__ import annotations

import uuid

from core.tests.factories import make

from maintainers.projectpolicies.models import ProjectPolicy


def make_policy(name: str = "Convencion de commits", *, project_id=None,
                slug: str | None = None, kind: str = "convention",
                body_md: str = "Commits en español.", is_active: bool = True,
                override_platform: bool = False, deleted: bool = False) -> ProjectPolicy:
    p = make(
        ProjectPolicy,
        project_id=project_id or uuid.uuid4(),
        slug=slug or name.lower().replace(" ", "-"),
        name=name,
        kind=kind,
        body_md=body_md,
        is_active=is_active,
        override_platform=override_platform,
        source="dashboard",
    )
    if deleted:
        from django.utils import timezone
        p.deleted_at = timezone.now()
        p.is_active = False
        p.save()
    return p
