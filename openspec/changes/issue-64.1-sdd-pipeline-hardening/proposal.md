# issue-64.1 — Endurecer el pipeline SDD

## Why

Tres contratos del pipeline SDD están rotos o incompletos y ya causaron
reintentos en corridas reales (documentado en la guía fable-5, flow a2056b29):

- **R3**: el parser `ParseScenarios` exige `## Scenario:` + bullets, pero la
  policy `openspec-spec-format` y el prompt de `sdd-spec` piden `#### Scenario:`
  sin bullets. El LLM sigue la policy → genera un spec que el parser rechaza.
- **R2**: `sdd-tasks` persiste las tasks en BD pero descarta los IDs generados,
  así el cliente no puede escribir los marcadores `<!-- t:uuid -->` en tasks.md.
  El round-trip queda roto y el apply ignora en silencio las tasks sin marcador.
- **R7**: `ApplyResult` mezcla archivos omitidos del array con conflictos de
  hash reales; el usuario no sabe si tiene que reenviar un archivo o resolver
  un conflicto.

## Scope

Entra: parser de escenarios (aceptar ambas variantes), seeds de policy y prompt,
handler de sdd-tasks (devolver IDs), applyTasks (reportar ignoradas), ApplyResult
(distinguir not_sent/unknown_issue/conflict).

Fuera: R4 (payload obeso), R5 (output_schema por step), R1 (pivot), R6 (modo
doc), R8 (bootstrap batch) — son issues separadas.

## Approach

- **R3**: ampliar el reconocimiento de heading (`## ` y `#### `) y de las líneas
  Given/When/Then (plano, `- `, `- **...**`) en `parse.go`. Alinear el texto de
  los seeds. Enriquecer el error de `engine.go` con un ejemplo.
- **R2**: capturar el retorno de `CreateTasks`, propagarlo a `PhaseResultResult`
  vía un nuevo campo `created_task_ids`. Contar y reportar tasks sin marcador en
  `applyTasks` / `ApplyResult`.
- **R7**: extender `ApplyResult` con clasificación por archivo y ajustar
  `applyFiles` + el mensaje de `issue_id inválido`.

## Risks

- Cambiar `ApplyResult` puede afectar a clientes que ya parsean su shape actual
  (mitigación: campos aditivos, no romper `Applied`/`Conflicts`).
- Ampliar el parser debe no romper specs ya válidas (mitigación: tests de
  regresión con ambos formatos).

## Testing

TDD en `internal/service/openspec/` (parse y engine) y en
`internal/service/orchestrator/` (phase_result). `go test` + `go vet`, sin build.
