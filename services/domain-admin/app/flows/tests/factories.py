"""Helpers para crear filas reales en la DB de test.

Los PKs son UUID (en prod los genera domain-mcp), así que en tests hay
que pasarlos explícitamente. organization_id también es un uuid explícito.
"""
from __future__ import annotations

import uuid

from flows.models import Flow, FlowVersion

# Org por defecto compartida entre helpers, para que los slugs choquen
# (la unicidad real es por (organization_id, slug)).
DEFAULT_ORG = uuid.UUID("11111111-1111-1111-1111-111111111111")


def make_flow(
    name: str,
    *,
    slug: str | None = None,
    organization_id: uuid.UUID | str = DEFAULT_ORG,
    description: str = "",
    spec: dict | None = None,
    is_active: bool = True,
    deterministic_replay: bool = False,
    seed_managed: bool = False,
    seed_version: int | None = None,
    deleted: bool = False,
) -> Flow:
    if slug is None:
        slug = name.lower().replace(" ", "-")
    f = Flow.objects.create(
        id=uuid.uuid4(),
        organization_id=organization_id,
        name=name,
        slug=slug,
        description=description,
        spec=spec if spec is not None else {"steps": []},
        is_active=is_active,
        deterministic_replay=deterministic_replay,
        seed_managed=seed_managed,
        seed_version=seed_version,
    )
    if deleted:
        from django.utils import timezone
        f.deleted_at = timezone.now()
        f.is_active = False
        f.save()
    return f


def make_flow_version(
    flow: Flow,
    *,
    version: int = 1,
    definition: dict | None = None,
    hash: str | None = None,
    note: str = "",
    created_by: uuid.UUID | None = None,
) -> FlowVersion:
    if hash is None:
        # Hash único determinista por (flow, version) para no chocar con
        # la unicidad (flow_id, hash).
        hash = f"{flow.pk.hex}{version:04d}".ljust(64, "0")[:64]
    return FlowVersion.objects.create(
        id=uuid.uuid4(),
        flow=flow,
        version=version,
        definition=definition if definition is not None else {"steps": []},
        hash=hash,
        note=note,
        created_by=created_by,
    )
