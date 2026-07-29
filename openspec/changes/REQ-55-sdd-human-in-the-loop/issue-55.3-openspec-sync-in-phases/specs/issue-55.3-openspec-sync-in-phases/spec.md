# issue-55.3-openspec-sync-in-phases — Spec

Como plataforma, el sync openspec BD→repo es contrato de fase (propose/design/tasks), no una nota en el protocolo global.

## ADDED Requirements

### Requirement: sync en el contrato de fase
sdd-propose/design/tasks DEBEN declarar required_tool_calls con domain_openspec_export + domain_openspec_apply, y su prompt DEBE instruir el loop export→escribir .md→apply.

#### Scenario: fase sin sync se rechaza
- **GIVEN** sdd-design completa sin llamar openspec_export/apply
- **WHEN** reporta phase_result
- **THEN** el server rechaza con missing_tool_calls

#### Scenario: sync cumplido
- **WHEN** reporta con export+apply en tool_calls
- **THEN** la fase cierra y openspec/ queda sincronizado
