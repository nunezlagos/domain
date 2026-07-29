# issue-54.1 required-tool-calls — Spec

Como orquestador SDD, cada fase declara las tools que el cliente DEBE haber invocado, y
rechazo el cierre de la fase si el reporte no las incluye, de modo que el mapeo fase→tools
sea un contrato verificable y no una sugerencia en prosa.

## ADDED Requirements

### Requirement: contrato de tools por fase
El sistema DEBE permitir que cada fase del pipeline SDD declare una lista de
`required_tool_calls` (nombres de tools `domain_*`), con un default en el handler Go y un
override opcional en `agent_templates.required_tool_calls`.

#### Scenario: override de plantilla gana sobre default del handler
- **GIVEN** la fase `sdd-verify` con default de handler `["domain_verify_start"]`
- **AND** `agent_templates.required_tool_calls` para `sdd-verify` = `["domain_verify_complete"]`
- **WHEN** el orquestador resuelve el contrato de la fase
- **THEN** el contrato efectivo es `["domain_verify_complete"]`

### Requirement: reporte de tool_calls en phase_result
`domain_orchestrate_phase_result` DEBE aceptar un campo opcional `tool_calls: []string`
donde el cliente reporta las tools que invocó durante la fase.

#### Scenario: cliente reporta las tools llamadas
- **GIVEN** un step en ejecución de la fase `sdd-verify`
- **WHEN** el cliente llama `domain_orchestrate_phase_result` con
  `tool_calls: ["domain_verify_start", "domain_verify_complete"]`
- **THEN** el servidor registra esas tools como reportadas para el step

### Requirement: rechazo de fase con contrato incumplido
El servidor DEBE rechazar el cierre de una fase cuando el contrato
`required_tool_calls` no es un subconjunto de las `tool_calls` reportadas, sin marcar el
step como `completed` ni como `failed` (el rechazo es reintentable).

#### Scenario: faltan tools requeridas
- **GIVEN** la fase `sdd-verify` con contrato `["domain_verify_start", "domain_verify_complete"]`
- **WHEN** el cliente reporta `tool_calls: ["domain_verify_start"]`
- **THEN** el step NO pasa a `completed`
- **AND** la respuesta incluye `missing_tool_calls: ["domain_verify_complete"]`
- **AND** el step permanece en estado `running` (reintentable)

#### Scenario: contrato cumplido avanza la fase
- **GIVEN** la fase `sdd-verify` con contrato `["domain_verify_start", "domain_verify_complete"]`
- **WHEN** el cliente reporta `tool_calls: ["domain_verify_start", "domain_verify_complete", "domain_mem_save"]`
- **THEN** el step pasa a `completed`
- **AND** el orquestador calcula el siguiente step

### Requirement: retrocompatibilidad
Un contrato `required_tool_calls` vacío DEBE comportarse como no-op (comportamiento
idéntico al actual), de modo que la activación sea incremental por fase.

#### Scenario: fase sin contrato cierra como hoy
- **GIVEN** la fase `sdd-explore` con `required_tool_calls` vacío
- **WHEN** el cliente llama `domain_orchestrate_phase_result` sin `tool_calls`
- **THEN** el step pasa a `completed` sin validación de contrato
