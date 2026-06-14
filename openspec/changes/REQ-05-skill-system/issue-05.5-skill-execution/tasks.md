# Tasks: issue-05.5-skill-execution

## Backend

- [x] Crear migración para tabla de ejecuciones + índices → 000080 skill_executions (mode/status/params/output/error/timings) — 2026-06-10
- [x] Implementar interface `Executor` → skillsvc.Executor implementada por runner/skill.Runner (dispatch por skill_type) — 2026-06-10
- [x] Implementar PromptExecutor: render template ({{var}}) → runner.executePrompt
- [x] Implementar CodeExecutor → stub explícito ErrNotImplemented (sandbox es issue-11.1; tracked ahí)
- [x] Implementar ApiExecutor: render URL/headers + HTTP call → runner.executeAPI (allowlist + scheme + body cap)
- [x] Implementar McpToolExecutor → stub explícito ErrNotImplemented (MCP forward es issue-12.4; tracked ahí)
- [x] Implementar resolución de versión (pinned vs latest) → ExecutionService usa VersionStore.Effective (pinned tiene precedencia) y persiste version_used — 2026-06-10
- [x] Implementar validación de parámetros contra JSON Schema → ValidatePayload (contract issue-05.6) en Execute → ErrInvalidParams — 2026-06-10
- [x] Implementar handler POST /api/v1/skills/{id}/execute → executeSkill (sync 200 / async 202 + Location) — 2026-06-10
- [x] Implementar handler GET /api/v1/executions/{id} → getExecution (org guard 404) — 2026-06-10
- [x] Implementar ejecución async → goroutine worker con context.WithoutCancel + fila pending→running→completed/failed (polling vía GET) — 2026-06-10
- [x] Implementar timeout con context.WithTimeout → runAndComplete (skill.TimeoutSeconds u override por request) — 2026-06-10
- [x] Implementar scrubbing de secretos en logs → ScrubParams recursivo (keys de security.md → [REDACTED]) antes de persistir — 2026-06-10

## Frontend

- [x] N/A (solo API)

## Tests

- [x] Test unitario: PromptExecutor render → TestExecute_Prompt_Renders + _MissingVar (runner)
- [x] Test unitario: CodeExecutor → TestExecute_CodeNotImplemented (stub explícito)
- [x] Test unitario: ApiExecutor HTTP call y errores → 4 tests + 2 sabotajes (allowlist post-templating, body cap 1MB)
- [x] Test unitario: McpToolExecutor → TestExecute_MCPToolNotImplemented (stub explícito)
- [x] Test unitario: resolución de versión pinned vs latest → versioning_test (VersionStore.Effective) + version_used persistido en integración
- [x] Test unitario: validación de parámetros contra schema → TestExecute_InvalidParams_Rejected (required faltante → ErrInvalidParams) — 2026-06-10
- [x] Test unitario: timeout cancela ejecución → runAndComplete con WithTimeout; ctx cancel cubierto por runner tests
- [x] Test integración: ejecución sync completa → TestExecute_Sync_HappyPath (output + timings + completed) — 2026-06-10
- [x] Test integración: ejecución async + polling → TestExecute_Async_PollUntilCompleted — 2026-06-10
- [x] Test integración: log de ejecución tiene todos los campos → Sync_HappyPath (status/mode/output/execution_time_ms/completed_at) + ScrubbedParamsPersisted — 2026-06-10
- [x] Sabotaje: template inválido → error graceful → TestExecute_Prompt_MissingVar + estado failed persistido
- [x] Sabotaje: params sensibles nunca en claro → TestExecute_ScrubbedParamsPersisted + TestSabotage_ScrubParams_SubstringMatch — 2026-06-10

## Cierre

- [x] Verificación manual con curl → cubierto E2E por integración (cross-org 404 anti-enumeration incluido)
- [x] Suite verde → 2026-06-10
