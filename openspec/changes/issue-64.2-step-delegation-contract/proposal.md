# issue-64.2 — Contrato de delegación por step (R5)

## Why

El orquestador SDD delega fases al cliente sin decirle upfront qué necesita, y
lo valida de a uno. Medido en vivo (flow issue-64.1): ~8 rechazos secuenciales
(intent → issue_md → proposal_md → task[].id → sabotage_records → judge_verdicts
→ verdict → skipped). Cada uno es un reintento completo. Dos causas:

- **A**: `PhaseStepSummary` no expone `required_tool_calls` ni `output_schema`,
  así el cliente no conoce el contrato antes de ejecutar.
- **B**: la validación corta al primer error (`handler.Validate` retorna el
  primer `ValidationError`; `ValidateRequiredSaves` corta en el primer missing).

## Scope

Entra: `PhaseStepSummary` (campos nuevos), `exportPlan`, definición de contrato
por fase en `phases/registry.go`, y la validación agregada en `phase_result.go`.

Fuera: R4 (payload obeso), R1/R6/R8.

## Approach

- **A**: agregar `RequiredToolCalls []string` y `OutputSchema map[string]any` a
  `PhaseStepSummary`; poblarlos desde la definición de fase; exportarlos en
  `exportPlan`. `required_tool_calls` ya se persiste en `step.Inputs`; falta
  exportarlo. `output_schema` se define por fase.
- **B**: refactor de la validación para acumular en una estructura los tres
  tipos de faltante (output fields, required_saves, tool_calls) y devolverlos
  juntos, sin cortar en el primero.

## Risks

- Cambiar el shape de la respuesta de validación puede afectar clientes que
  parsean `validation_error` como string único (mitigación: mantener el string
  agregado + agregar campos estructurados aditivos).
- Definir `output_schema` por fase es trabajo repetitivo; empezar por las fases
  que más rechazan (spec, tasks, judge).

## Testing

TDD en `internal/service/orchestrator/`. `go test` + `go vet`, sin build.
