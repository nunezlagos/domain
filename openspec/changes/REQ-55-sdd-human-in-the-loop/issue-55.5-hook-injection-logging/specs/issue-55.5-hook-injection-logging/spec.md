# issue-55.5-hook-injection-logging — Spec

Como usuario, puedo auditar en un archivo qué additionalContext inyectó cada hook (la UI de Claude Code no lo muestra).

## ADDED Requirements

### Requirement: log de inyecciones
Los hooks de domain DEBEN loguear a ~/.local/state/domain/injections.log cada additionalContext emitido (timestamp, session_id, hook, resumen).

#### Scenario: inyección registrada
- **GIVEN** el hook UserPromptSubmit emite additionalContext
- **WHEN** corre
- **THEN** agrega una línea al log con timestamp, session, hook y resumen

#### Scenario: best-effort
- **GIVEN** el log no se puede escribir
- **WHEN** el hook corre
- **THEN** no falla ni bloquea la sesión (exit 0)
