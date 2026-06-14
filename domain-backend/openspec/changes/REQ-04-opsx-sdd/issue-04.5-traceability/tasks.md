# Tasks: issue-04.5-traceability

## Backend

- [x] `migrations/XXXX_create_code_references.sql`: tabla + UNIQUE(issue_id, file_path) + FK
- [x] `internal/opsx/traceability.go`: structs `RequirementTrace`, `CodeTrace`, `CoverageDashboard`, `ProgressReport`, `ConsolidatedRow`
- [x] `internal/store/pg/code_reference.go`: interfaz `CodeReferenceStore` + CRUD
- [x] `internal/opsx/traceability_service.go`: `TraceabilityService` interfaz
- [x] Implementar `GetRequirementTrace(slug string) (*RequirementTrace, error)` con joins a HU→Proposal→Design→Tasks→CodeRefs
- [x] Implementar `GetCodeTrace(filePath string) (*CodeTrace, error)` backward: file→HU→REQ
- [x] Implementar `GetCoverageDashboard() (*CoverageDashboard, error)` con COUNT + FILTER
- [x] Implementar `GetProgressReport() ([]ProgressReport, error)` con GROUP BY req_id
- [x] Implementar `GetHUsWithoutProposals() ([]Issue, error)` LEFT JOIN + IS NULL
- [x] Implementar `GetHUsWithoutDesigns() ([]Issue, error)`
- [x] Implementar `GetHUsWithIncompleteTasks() ([]Issue, error)`
- [x] Implementar `GetConsolidatedReport() ([]ConsolidatedRow, error)` matriz completa

## Tests

- [x] Test unitario de estructura de reportes (formato correcto)
- [x] Test de integración: crear datos en todas las tablas → RequirementTrace completo
- [x] Test de integración: CodeTrace backward
- [x] Test de integración: CoverageDashboard con datos parciales
- [x] Test de integración: cross-references (sin proposal, sin design)
- [x] Test de integración: ConsolidatedReport con 2 REQs
- [x] Test con 0 datos: todos los reportes devuelven vacío/0 sin error
- [x] Sabotaje: eliminar todas las tablas → error claro (no panic)

## Cierre

- [x] Verificación manual: ejecutar dashboard, verificar cobertura
- [x] Suite verde
