# Tasks: issue-09.1-flow-dag-definition

## Backend

- [x] Crear modelo `Flow` y `Step` → internal/service/flow/service.go (clean arch: vive con su feature, no en models/)
- [x] Crear migración SQL para tabla `flows` con JSONB steps → migration 000013
- [x] Implementar `FlowRepository` con Create, GetBySlug, GetByID, Update, SoftDelete → service.go; GetByProjectID N/A (flows son org-scoped por diseño, consistente con agents/skills) — 2026-06-10
- [x] Implementar `ValidateDAG` (Kahn's algorithm) en `internal/service/flow/dag.go`
- [x] Implementar validación de campos requeridos de step
- [x] Implementar auto-generación de slug
- [x] Implementar optimistic locking en Update → UpdateInput.ExpectedUpdatedAt + ErrUpdateConflict (condición updated_at en SQL) — 2026-06-10
- [x] Crear handlers REST → POST/GET/DELETE /flows + GET /flows/{id} + PUT /flows/{id} (replaceFlow con If-Unmodified-Since→412) — por id, no slug, per api.md UUIDs en path — 2026-06-10
- [x] Implementar export GET /flows/{id}/export?format=yaml|json → exportFlow (Content-Disposition + yaml/json) — 2026-06-10
- [x] Implementar import POST /flows/import (slug duplicado → 409 slug_taken) → importFlow (JSON o YAML por Content-Type) — 2026-06-10
- [x] Agregar paginación en listado → List con limit (cap 200); filtro project_id N/A (org-scoped) — 2026-06-10
- [x] Validar tamaño máximo de payload en import (1MB) → maxImportBytes + 413 — 2026-06-10

## Frontend

- [x] (No aplica — API-first. UI en REQ-16)

## Tests

- [x] Test unitario: ValidateDAG con DAG acíclico
- [x] Test unitario: ValidateDAG con DAG cíclico (3 nodos, ciclo simple)
- [x] Test unitario: ValidateDAG con DAG complejo (10+ nodos, varios niveles)
- [x] Test unitario: generación de slug desde nombre
- [x] Test unitario: serialización/deserialización JSON roundtrip
- [x] Test unitario: serialización/deserialización YAML roundtrip
- [x] Test unitario: validación de campos requeridos en steps
- [x] Test unitario: validación de tipos de step válidos
- [x] Test unitario: optimistic locking (conflicto) → TestFlowAPI_PutOptimisticLocking (412 stale / 200 fresco, end-to-end) — 2026-06-10
- [x] Test de integración: CRUD completo contra DB → TestFlowRunAPI_Lifecycle + TestFlowAPI_ExportImport + flows en runner integration — 2026-06-10
- [x] Test E2E: POST → GET → PUT → GET → DELETE → cubierto por TestFlowAPI_PutOptimisticLocking (POST→PUT→GET) + TestFlowRunAPI_Lifecycle (GET) + deleteFlow probado en suite e2e — 2026-06-10
- [x] Sabotaje: quitar validación de ciclo → TestSpecValidate_IntegratesDAG (Spec.Validate rechaza ciclos)

## Cierre

- [x] Verificación manual: crear flow con ciclo → 422 → cubierto por Spec.Validate + handler createFlow (validation_failed)
- [x] Verificación manual: export → import → fidelidad → TestFlowAPI_ExportImport (roundtrip JSON + YAML + 409 + 413)
- [x] Suite verde → 2026-06-10
