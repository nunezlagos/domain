# Proposal: issue-09.1-flow-dag-definition

## Intención

Proveer un CRUD completo para definir flujos de trabajo como DAGs (Directed Acyclic Graphs). Cada flow tiene nombre, slug único por proyecto, descripción, lista ordenada de pasos con dependencias y un versionado incremental. Se debe validar que el grafo sea acíclico, que todos los campos requeridos estén presentes y que los tipos de step sean válidos. Se debe soportar import/export en YAML y JSON.

## Scope

**Incluye:**
- CRUD REST de flows: POST, GET (individual y listado), PUT, DELETE
- Validación de DAG: detección de ciclos vía topological sort (Kahn's algorithm)
- Validación de schema de steps: campos requeridos (id, type), tipos válidos
- Auto-generación de slug a partir del nombre
- Versionado automático (incrementa en cada update)
- Export GET /flows/:slug/export?format=yaml|json
- Import POST /flows/import con detección de conflictos por slug
- Paginación y filtrado por project_id

**Excluye:**
- Ejecución de flows (issue-09.3)
- Validación semántica de parámetros específicos por step type (issue-09.2)
- Composición de sub-flows (issue-09.5)

## Enfoque técnico

- Modelo `Flow` en `internal/models/flow.go` con fields: ID, Name, Slug, Description, ProjectID, Steps ([]Step), Version, CreatedAt, UpdatedAt
- Step: ID, Type (enum string), Params (jsonb), DependsOn ([]string), optional Label, optional Timeout
- Validación de DAG: Kahn's algorithm para topological sort en `internal/flow/validator.go`
- Repositorio en `internal/database/flow_repo.go` con queries parametrizadas
- Handler REST en `internal/api/handlers/flow.go`
- Serialización YAML vía `gopkg.in/yaml.v3`
- Slug generado con `slug.Make` del paquete `github.com/gosimple/slug`

## Riesgos

- DAGs con muchos steps (>1000) pueden ser costosos de validar → límite práctico de 500 steps por flow.
- La importación YAML/JSON puede recibir payloads maliciosos → validar tamaño máximo (1MB) y schema estricto.
- Versionado: updates concurrentes pueden causar conflictos de versión → usar optimistic locking con `version` field y check en UPDATE WHERE version = :old.

## Testing

- Unit: validación de DAG cíclico/acyclico, validación de campos requeridos, generación de slug
- Unit: serialización/deserialización roundtrip YAML y JSON
- Integration: CRUD contra DB real (testcontainer)
- E2E: POST → GET → PUT → export → import → DELETE cycle
- Sabotaje: romper un step requerido → test cae
