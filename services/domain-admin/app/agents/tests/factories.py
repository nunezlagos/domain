"""Helpers para crear filas reales en la DB de test.

Los PKs uuid (agents, agent_templates) en prod los genera domain-mcp, así
que en tests hay que pasarlos explícitamente. agent_versions tiene PK
BIGSERIAL (autoincremental): NO se pasa id.

organization_id es un uuid explícito; DEFAULT_ORG compartida para que los
slugs choquen (la unicidad real es por (organization_id, slug)).
"""
from __future__ import annotations

import uuid

from agents.models import Agent, AgentTemplate, AgentVersion

DEFAULT_ORG = uuid.UUID("11111111-1111-1111-1111-111111111111")


def make_agent(
    name: str,
    *,
    slug: str | None = None,
    organization_id: uuid.UUID | str = DEFAULT_ORG,
    provider: str = "anthropic",
    model: str = "claude-haiku-4-5",
    description: str = "",
    system_prompt: str = "",
    skills_slugs: list[str] | None = None,
    max_iterations: int = 20,
    token_budget: int | None = None,
    temperature=None,
    seed_managed: bool = False,
    is_user_modified: bool = False,
    deleted: bool = False,
) -> Agent:
    if slug is None:
        slug = name.lower().replace(" ", "-")
    a = Agent.objects.create(
        id=uuid.uuid4(),
        organization_id=organization_id,
        name=name,
        slug=slug,
        provider=provider,
        model=model,
        description=description,
        system_prompt=system_prompt,
        skills_slugs=skills_slugs or [],
        max_iterations=max_iterations,
        token_budget=token_budget,
        temperature=temperature,
        seed_managed=seed_managed,
        is_user_modified=is_user_modified,
    )
    if deleted:
        from django.utils import timezone
        a.deleted_at = timezone.now()
        a.save()
    return a


def make_agent_version(
    agent: Agent,
    version: int,
    *,
    snapshot: dict | None = None,
    changed_by: uuid.UUID | str | None = None,
) -> AgentVersion:
    # PK BIGSERIAL → no se pasa id.
    return AgentVersion.objects.create(
        agent=agent,
        version=version,
        snapshot=snapshot or {},
        changed_by=changed_by,
    )


def make_agent_template(
    name: str,
    *,
    slug: str | None = None,
    organization_id: uuid.UUID | str = DEFAULT_ORG,
    system_prompt: str = "Sos un agente de prueba.",
    role: str = "phase-worker",
    handoff_policy: str = "allow",
    capabilities: list[str] | None = None,
) -> AgentTemplate:
    if slug is None:
        slug = name.lower().replace(" ", "-")
    return AgentTemplate.objects.create(
        id=uuid.uuid4(),
        organization_id=organization_id,
        name=name,
        slug=slug,
        system_prompt=system_prompt,
        role=role,
        handoff_policy=handoff_policy,
        capabilities=capabilities or [],
    )
