# issue-54.5 parallel-phase-subagents — Spec

Como agente cliente ejecutando una fase del SDD, recibo en el prompt un plan de
subagentes paralelos (roles, contexto, merge) para fanear mis subagentes en las
fases que se benefician, sin cambiar el modelo económico client-driven.

## ADDED Requirements

### Requirement: plan de subagentes por fase
El sistema DEBE permitir que cada fase declare un `subagent_plan` (default en
el handler, override en `agent_templates.metadata.subagent_plan`) y DEBE
inyectarlo en el user_prompt de la fase tanto en el plan inicial como en el
lazy rebuild.

#### Scenario: el plan llega al prompt de la fase
- **GIVEN** la fase `sdd-judge` con subagent_plan declarado
- **WHEN** el orquestador entrega el prompt de la fase (plan inicial o rebuild)
- **THEN** el user_prompt contiene el bloque "Plan de subagentes"
- **AND** el bloque instruye lanzar los jueces EN PARALELO y cómo mergear

#### Scenario: override de template gana
- **GIVEN** `agent_templates.metadata.subagent_plan` configurado para la fase
- **WHEN** se resuelve el plan efectivo
- **THEN** el del template reemplaza al default del handler

#### Scenario: fase sin plan es no-op
- **GIVEN** una fase sin subagent_plan (default vacío, sin override)
- **WHEN** se entrega su prompt
- **THEN** el prompt no contiene el bloque (comportamiento actual intacto)

### Requirement: el shape del Output exige el paralelismo donde es crítico
En las fases donde el fan-out es la esencia (judge), el `handler.Validate`
DEBE rechazar outputs que no puedan provenir del paralelismo declarado.

#### Scenario: un solo juez es rechazado
- **GIVEN** la fase `sdd-judge` con plan de panel adversarial
- **WHEN** el cliente reporta `judge_verdicts` con un solo veredicto
- **THEN** el server rechaza el cierre de la fase

#### Scenario: panel completo pasa
- **WHEN** el cliente reporta `judge_verdicts` con 2+ veredictos (roles
  correctness y security)
- **THEN** la fase se cierra normalmente

### Requirement: el contexto preparado alimenta a los subagentes
El bloque del plan DEBE referenciar el `prepared_context` (issue-54.2) para
que el agente distribuya a cada subagente el contexto pertinente a su rol.

#### Scenario: subagentes reciben contexto
- **GIVEN** la fase `sdd-explore` con prepared_context (memoria del proyecto)
  y subagent_plan (N áreas)
- **WHEN** el agente fanea los Explore
- **THEN** el prompt instruye pasar a cada subagente el contexto de su área
