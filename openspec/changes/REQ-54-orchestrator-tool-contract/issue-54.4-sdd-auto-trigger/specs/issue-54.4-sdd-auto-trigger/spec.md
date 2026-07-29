# issue-54.4 sdd-auto-trigger — Spec

Como agente cliente, cuando el usuario pide un requerimiento recibo una señal
determinista que me instruye a orquestar con domain_orchestrate antes de
implementar, de modo que el SDD sea el camino default.

## ADDED Requirements

### Requirement: clasificación del prompt en el capture
`domain_prompt_capture` DEBE responder, además del id, una `classification`
con `complexity` (trivial|simple|moderate|complex), `suggested_action`
(none|ticket|orchestrate|resume) y `suggested_mode`, calculada con el
heurístico léxico existente (sin LLM).

#### Scenario: prompt de feature clasifica orchestrate
- **GIVEN** un prompt "implementar un nuevo módulo de pagos con migración"
- **WHEN** el hook lo captura vía domain_prompt_capture
- **THEN** la respuesta incluye `classification.complexity = "complex"`
- **AND** `classification.suggested_action = "orchestrate"`

#### Scenario: prompt trivial no sugiere orquestar
- **GIVEN** un prompt "fix typo en el README"
- **WHEN** se captura
- **THEN** `classification.suggested_action` es `"none"` o `"ticket"`

### Requirement: flow activo sugiere retomar, no re-orquestar
Cuando el proyecto tiene un flow_run en estado no-terminal, la clasificación
DEBE devolver `suggested_action = "resume"` con el `active_flow_run_id`.

#### Scenario: hay un flow corriendo
- **GIVEN** un proyecto con un flow_run en estado running
- **WHEN** se captura cualquier prompt no-trivial del proyecto
- **THEN** `classification.suggested_action = "resume"`
- **AND** la respuesta incluye `active_flow_run_id`

### Requirement: el hook inyecta la señal como additionalContext
El hook UserPromptSubmit DEBE emitir `hookSpecificOutput.additionalContext`
instruyendo al agente cuando `suggested_action` es `orchestrate` o `resume`,
y NO DEBE emitir nada (ni bloquear) en los demás casos.

#### Scenario: señal de orquestación inyectada
- **GIVEN** el hook recibe un prompt que clasifica complex
- **WHEN** procesa la respuesta del capture
- **THEN** imprime a stdout un JSON con `hookSpecificOutput.hookEventName = "UserPromptSubmit"`
- **AND** `additionalContext` contiene "domain_orchestrate"

#### Scenario: prompt trivial pasa limpio
- **GIVEN** el hook recibe un prompt trivial
- **WHEN** procesa la respuesta
- **THEN** no emite additionalContext
- **AND** sale con exit 0 (el prompt nunca se bloquea)

### Requirement: la señal es sugerencia, no bloqueo
El hook NUNCA DEBE usar `decision: "block"`; el usuario siempre puede ordenar
implementar sin SDD y el agente debe obedecer.

#### Scenario: el usuario pide salteo explícito
- **GIVEN** la señal de orquestación fue inyectada
- **WHEN** el usuario dice "hacelo directo sin SDD"
- **THEN** el agente procede sin orquestar (la señal no es vinculante)
