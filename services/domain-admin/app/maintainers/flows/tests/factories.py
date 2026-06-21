"""Factories del mantenedor de Flows.

Reusa core.tests.factories.make (pone un PK uuid si no se pasa `id`, ya que en
prod los genera domain-mcp). Solo agrega los helpers específicos de las 2
tablas del dominio (flows, flow_versions).
"""
from __future__ import annotations

import uuid

from core.tests.factories import make

from maintainers.flows.models import Flow, FlowVersion


def make_flow(
    name: str,
    *,
    slug: str | None = None,
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
    f = make(
        Flow,
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
    return make(
        FlowVersion,
        flow=flow,
        version=version,
        definition=definition if definition is not None else {"steps": []},
        hash=hash,
        note=note,
        created_by=created_by,
    )
