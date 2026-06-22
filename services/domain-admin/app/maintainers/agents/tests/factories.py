"""Factories del mantenedor de Agentes.

Reusa core.tests.factories.make (pone un PK uuid si no se pasa `id`) para los
models con PK uuid (Agent, AgentTemplate). AgentVersion tiene PK BIGSERIAL
(autoincremental) -> NO se pasa id, se crea directo.
"""
from __future__ import annotations

import uuid

from core.tests.factories import make

from maintainers.agents.models import Agent, AgentTemplate, AgentVersion


def make_agent(
    name: str,
    *,
    slug: str | None = None,
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
    a = make(
        Agent,
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
    # PK BIGSERIAL -> no se pasa id (make() pondria un uuid).
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
    system_prompt: str = "Sos un agente de prueba.",
    role: str = "phase-worker",
    handoff_policy: str = "allow",
    capabilities: list[str] | None = None,
) -> AgentTemplate:
    if slug is None:
        slug = name.lower().replace(" ", "-")
    return make(
        AgentTemplate,
        name=name,
        slug=slug,
        system_prompt=system_prompt,
        role=role,
        handoff_policy=handoff_policy,
        capabilities=capabilities or [],
    )
