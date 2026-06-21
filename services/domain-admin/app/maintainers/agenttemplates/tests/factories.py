"""Factories del mantenedor de Plantillas de Agentes.

Reusa core.tests.factories.make (pone un PK uuid si no se pasa `id`, ya que en
prod los genera domain-mcp). Solo agrega el helper de la tabla agent_templates.

Nota: el model es un proxy de maintainers.agents.AgentTemplate (misma tabla,
mismas columnas). `make` no choca con la columna `model` porque AgentTemplate
tiene id (positional-only `model` en make).
"""
from __future__ import annotations

from core.tests.factories import make

from maintainers.agenttemplates.models import AgentTemplate


def make_agent_template(
    name: str,
    *,
    slug: str | None = None,
    system_prompt: str = "Sos un agente.",
    personality: str = "",
    capabilities: list[str] | None = None,
    model: str = "claude-haiku-4-5",
    temperature=0.7,
    max_tokens: int = 4096,
    handoff_policy: str = "allow",
    role: str = "phase-worker",
    status: str = "active",
) -> AgentTemplate:
    if slug is None:
        slug = name.lower().replace(" ", "-")
    return make(
        AgentTemplate,
        name=name,
        slug=slug,
        system_prompt=system_prompt,
        personality=personality,
        capabilities=capabilities or [],
        model=model,
        temperature=temperature,
        max_tokens=max_tokens,
        handoff_policy=handoff_policy,
        role=role,
        status=status,
    )
