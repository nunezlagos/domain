# issue-55.2-exec-mode-choice — Spec

Como usuario, al iniciar un flow SDD elijo auto vs human-in-the-loop en vez de caer en auto por omisión.

## ADDED Requirements

### Requirement: elección de exec_mode al inicio
El sdd-orchestrator DEBE preguntar (AskUserQuestion) auto vs human-in-the-loop al iniciar, mapeando a exec_mode auto|hybrid.

#### Scenario: elección consciente
- **GIVEN** un flow que arranca
- **WHEN** el orquestador inicia
- **THEN** pregunta el modo antes de la primera fase

### Requirement: hardspec por default
hardspec DEBE ser true por default (spec siempre pausa a revisión).

#### Scenario: pausa tras spec
- **GIVEN** hardspec=true
- **WHEN** sdd-spec completa
- **THEN** el flujo pausa hasta domain_orchestrate_confirm
