"""Helpers para crear filas reales en la DB de test.

Los PKs son UUID (en prod los genera domain-mcp), así que en tests hay
que pasarlos explícitamente. organization_id y target_id también son
uuid explícitos.
"""
from __future__ import annotations

import uuid

from crons.models import Cron

# Org por defecto compartida entre helpers, para que los slugs choquen
# (la unicidad real es por (organization_id, slug)).
DEFAULT_ORG = uuid.UUID("11111111-1111-1111-1111-111111111111")
DEFAULT_TARGET = uuid.UUID("22222222-2222-2222-2222-222222222222")


def make_cron(
    name: str,
    *,
    slug: str | None = None,
    organization_id: uuid.UUID | str = DEFAULT_ORG,
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
    c = Cron.objects.create(
        id=uuid.uuid4(),
        organization_id=organization_id,
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
