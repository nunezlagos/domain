# issue-55.4-auto-skill-policy-bootstrap — Spec

Como usuario, al bootstrap veo las policies/skills candidatas de mis .md y confirmo cuáles importar y a qué scope.

## ADDED Requirements

### Requirement: candidatos en el bootstrap
domain_session_bootstrap DEBE, si hay .md sin importar (existing_rules_files no cubiertos por project_policies), devolver una señal + lista de candidatos a importar.

#### Scenario: candidatos detectados
- **GIVEN** un proyecto con AGENTS.md sin importar
- **WHEN** corre el bootstrap
- **THEN** la respuesta incluye candidatos con su clasificación sugerida

### Requirement: confirmación con scope antes de persistir
Los candidatos DEBEN presentarse con AskUserQuestion (confirmar/editar/descartar + scope proyecto vs platform) antes de persistir.

#### Scenario: nada sin OK
- **GIVEN** candidatos detectados
- **WHEN** el agente los procesa
- **THEN** no persiste ninguno sin confirmación explícita del usuario
