"""Factories del mantenedor de Crons.

Reusa core.tests.factories.make (pone un PK uuid si no se pasa `id`, ya que en
prod los genera domain-mcp). target_id tambien es uuid explicito.
"""
from __future__ import annotations

import uuid

from core.tests.factories import make

from maintainers.crons.models import Cron

DEFAULT_TARGET = uuid.UUID("22222222-2222-2222-2222-222222222222")


def make_cron(
    name: str,
    *,
    slug: str | None = None,
    cron_expression: str = "0 9 * * *",
    target_type: str = "flow",
    target_id: uuid.UUID | str = DEFAULT_TARGET,
    timezone: str = "UTC",
    description: str = "",
    inputs: dict | None = None,
    enabled: bool = True,
    deleted: bool = False,
) -> Cron:
    if slug is None:
        slug = name.lower().replace(" ", "-")
    c = make(
        Cron,
        created_by=None,
        name=name,
        slug=slug,
        description=description,
        cron_expression=cron_expression,
        timezone=timezone,
        target_type=target_type,
        target_id=target_id,
        inputs=inputs if inputs is not None else {},
        enabled=enabled,
    )
    if deleted:
        from django.utils import timezone as tz
        c.deleted_at = tz.now()
        c.enabled = False
        c.save()
    return c
