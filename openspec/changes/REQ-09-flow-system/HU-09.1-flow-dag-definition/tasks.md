# Tasks: HU-09.1-flow-dag-definition

## Backend

- [ ] Crear modelo `Flow` y `Step` en `internal/models/flow.go`
- [ ] Crear migración SQL para tabla `flows` con JSONB steps
- [ ] Implementar `FlowRepository` con Create, GetBySlug, GetByProjectID, Update, Delete
- [ ] Implementar `ValidateDAG` (Kahn's algorithm) en `internal/flow/validator.go`
- [ ] Implementar validación de campos requeridos de step
- [ ] Implementar auto-generación de slug
- [ ] Implementar optimistic locking en Update
- [ ] Crear handler REST: POST /flows, GET /flows, GET /flows/:slug, PUT /flows/:slug, DELETE /flows/:slug
- [ ] Implementar export GET /flows/:slug/export?format=yaml|json
- [ ] Implementar import POST /flows/import (con detección de slug duplicado)
- [ ] Agregar paginación y filtro por project_id en listado
- [ ] Validar tamaño máximo de payload en import (1MB)

## Frontend

- [ ] (No aplica — API-first. UI en REQ-16)

## Tests

- [ ] Test unitario: ValidateDAG con DAG acíclico
- [ ] Test unitario: ValidateDAG con DAG cíclico (3 nodos, ciclo simple)
- [ ] Test unitario: ValidateDAG con DAG complejo (10+ nodos, varios niveles)
- [ ] Test unitario: generación de slug desde nombre
- [ ] Test unitario: serialización/deserialización JSON roundtrip
- [ ] Test unitario: serialización/deserialización YAML roundtrip
- [ ] Test unitario: validación de campos requeridos en steps
- [ ] Test unitario: validación de tipos de step válidos
- [ ] Test unitario: optimistic locking (conflicto de versión)
- [ ] Test de integración: CRUD completo contra DB
- [ ] Test E2E: POST → GET → PUT → GET → DELETE
- [ ] Sabotaje: quitar validación de ciclo → test de ciclo falla

## Cierre

- [ ] Verificación manual: crear flow con ciclo → 422
- [ ] Verificación manual: export → import → fidelidad
- [ ] Suite verde
