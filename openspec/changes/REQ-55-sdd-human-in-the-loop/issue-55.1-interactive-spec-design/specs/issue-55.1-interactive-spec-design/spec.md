# issue-55.1-interactive-spec-design — Spec

Como agente en spec/design, uso AskUserQuestion (opciones + texto libre) y estas fases nunca corren en subagente.

## ADDED Requirements

### Requirement: interactividad en spec/design
El system_prompt de sdd-spec y sdd-design DEBE instruir usar AskUserQuestion con opciones y opción de texto libre.

#### Scenario: el spec pregunta con opciones
- **GIVEN** la fase sdd-spec con ambigüedades
- **WHEN** el agente necesita clarificar
- **THEN** usa AskUserQuestion (opciones + 'Other'), no prosa

### Requirement: spec/design nunca en subagente
Estas fases NO DEBEN delegarse a subagentes (AskUserQuestion no disponible ahí).

#### Scenario: no-subagente
- **GIVEN** un plan de fan-out
- **WHEN** llega a spec o design
- **THEN** corre en el agente principal, no en subagente
