# Tasks: issue-04.5-traceability

## Backend

- [ ] `migrations/XXXX_create_code_references.sql`: tabla + UNIQUE(issue_id, file_path) + FK
- [ ] `internal/opsx/traceability.go`: structs `RequirementTrace`, `CodeTrace`, `CoverageDashboard`, `ProgressReport`, `ConsolidatedRow`
- [ ] `internal/store/pg/code_reference.go`: interfaz `CodeReferenceStore` + CRUD
- [ ] `internal/opsx/traceability_service.go`: `TraceabilityService` interfaz
- [ ] Implementar `GetRequirementTrace(slug string) (*RequirementTrace, error)` con joins a HU→Proposal→Design→Tasks→CodeRefs
- [ ] Implementar `GetCodeTrace(filePath string) (*CodeTrace, error)` backward: file→HU→REQ
- [ ] Implementar `GetCoverageDashboard() (*CoverageDashboard, error)` con COUNT + FILTER
- [ ] Implementar `GetProgressReport() ([]ProgressReport, error)` con GROUP BY req_id
- [ ] Implementar `GetHUsWithoutProposals() ([]Issue, error)` LEFT JOIN + IS NULL
- [ ] Implementar `GetHUsWithoutDesigns() ([]Issue, error)`
- [ ] Implementar `GetHUsWithIncompleteTasks() ([]Issue, error)`
- [ ] Implementar `GetConsolidatedReport() ([]ConsolidatedRow, error)` matriz completa

## Tests

- [ ] Test unitario de estructura de reportes (formato correcto)
- [ ] Test de integración: crear datos en todas las tablas → RequirementTrace completo
- [ ] Test de integración: CodeTrace backward
- [ ] Test de integración: CoverageDashboard con datos parciales
- [ ] Test de integración: cross-references (sin proposal, sin design)
- [ ] Test de integración: ConsolidatedReport con 2 REQs
- [ ] Test con 0 datos: todos los reportes devuelven vacío/0 sin error
- [ ] Sabotaje: eliminar todas las tablas → error claro (no panic)

## Cierre

- [ ] Verificación manual: ejecutar dashboard, verificar cobertura
- [ ] Suite verde
