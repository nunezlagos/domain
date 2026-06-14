# Workflow Versioning — Flows

> issue-09.7 — `internal/service/flow/versioning.go` + `internal/runner/flow/versionpin.go`

Cada flow tiene versiones inmutables de su definition. Los runs quedan
pinneados a la versión con la que arrancaron: cambiar el flow nunca afecta
runs en vuelo.

## Modelo

- `flow_versions`: snapshot inmutable `{flow_id, version, definition, hash, status}`
- `status`: `draft` → `published` → `deprecated` (migración 000077)
- `is_default`: única versión default por flow (unique index parcial)
- `flow_runs.flow_version_id`: versión congelada al iniciar el run

## Lifecycle

| Acción | Efecto |
|--------|--------|
| `PATCH /api/v1/flows/:id` (fv-002) | Crea versión **draft** v(n+1) con el spec propuesto; la current queda intacta |
| `PublishVersion` (fv-003) | `status=published` + flip transaccional de `is_default` |
| `DeprecateVersion` (fv-004) | `status=deprecated`; invocarla → rechazo (410 a nivel API) |
| Cron diario (fv-009) | `ArchiveDeprecated`: borra deprecated >90d sin runs que las referencien |

Solo `published` es invocable (`IsVersionInvokable`); draft y deprecated se
rechazan.

## Run pinning (fv-008)

- Al iniciar un run, el engine asegura una versión cuyo definition coincida
  con el spec actual (idempotente por hash: `FindByHash` → `NewVersion`) y
  guarda `flow_runs.flow_version_id`.
- `RunInput.FlowVersion` permite invocar una versión específica publicada.
- En resume (issue-09.6), el engine carga la definition desde la versión
  pinneada del run — NUNCA la current del flow.

## Versionado idempotente

`NewVersion` no crea duplicados: si el hash coincide con la última versión,
retorna la existente. El pin usa `FindByHash` (cualquier posición) para no
inflar versiones cuando current y un draft viejo coexisten.

## Diff y breaking changes

`DiffVersions(from, to)` retorna cambios estructurales (`added/removed/
modified` por path) con flag `breaking` (step removido, type cambiado).

## Tests

`internal/runner/flow/version_integration_test.go`: pin idempotente,
draft no invokable, archive conserva versiones referenciadas.
`internal/service/flow/versioning_test.go`: lifecycle completo.
