"""Modelo del mantenedor de Plantillas de Agentes (tabla agent_templates).

La tabla `agent_templates` (managed=False; Django solo lee/escribe vía ORM) YA
está mapeada por maintainers.agents.models.AgentTemplate, donde se declaran las
columnas REALES (que matchean EXACTO la tabla, SIN deleted_at → NO soft-delete):

    id, slug, name, system_prompt, personality, capabilities, model,
    temperature, max_tokens, handoff_policy, metadata, created_by, created_at,
    updated_at, role, seed_managed, is_user_modified, seed_version, status

Las columnas comunes (id uuid / created_at / updated_at) las aporta
core.models.BaseModel a través del modelo concreto de agents (que hereda de
BaseModel). NO hay deleted_at en la tabla, por eso el modelo concreto hereda de
BaseModel y NO de SoftDeleteModel: la baja de este mantenedor es HARD delete.

Por qué PROXY y no un segundo modelo concreto:
    Dos modelos `managed` sobre la MISMA db_table disparan models.E028 (system
    check error), y el runner de tests —que flipea managed=False→True— haría
    fallar el arranque del suite entero. Un proxy comparte tabla y columnas con
    el modelo concreto sin crear una segunda tabla ni disparar E028 (los proxies
    están exentos). Así NO tocamos maintainers.agents ni core, y este app suma
    SOLO el CRUD del mantenedor.
"""
from __future__ import annotations

from maintainers.agents.models import AgentTemplate as _AgentTemplate


class AgentTemplate(_AgentTemplate):
    """Plantilla de agente. Proxy sobre maintainers.agents.AgentTemplate.

    Misma tabla (agent_templates), mismas columnas reales. Solo agrega el
    contrato propio del mantenedor (ordering por `name`). Reusa los choices
    (HANDOFF_POLICY_CHOICES / ROLE_CHOICES / STATUS_CHOICES), las @property
    display_name y los métodos get_*_display del modelo concreto.
    """

    class Meta:
        proxy = True
        app_label = "agenttemplates"
        verbose_name = "Plantilla de agente"
        verbose_name_plural = "Plantillas de agentes"
        ordering = ["name"]
