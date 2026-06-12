# Tasks: issue-04.1-requirements-crud

## Backend

- [x] `migrations/XXXX_create_requirements.sql`: tabla + índices + self-FK
- [x] `internal/opsx/requirement.go`: structs `Requirement`, `RequirementFilter`, `RequirementTree`
- [x] `internal/store/pg/requirement.go`: interfaz `RequirementStore`
- [x] Implementar `Create(req Requirement) (uuid.UUID, error)` con validación de slug y parent
- [x] Implementar `GetBySlug(slug string) (*Requirement, error)`
- [x] Implementar `GetByID(id uuid.UUID) (*Requirement, error)`
- [x] Implementar `Update(slug string, req RequirementUpdate) error`
- [x] Implementar `Archive(slug string, recursive bool) error`
- [x] Implementar `List(filter RequirementFilter) ([]Requirement, int, error)`
- [x] Implementar `GetTree(slug string) (*RequirementTree, error)` con CTE recursiva
- [x] `internal/opsx/requirement_service.go`: lógica de negocio, validaciones

## Tests

- [x] Test unitario de validación de slug (formato correcto/incorrecto)
- [x] Test de integración: crear raíz + hijo + árbol
- [x] Test de integración: filtros combinados (status + priority)
- [x] Test de integración: archive sin cascade → hijos no se archivan
- [x] Test de integración: archive con cascade → todos archivados
- [x] Test de slug duplicado → error
- [x] Test de parent_id inexistente → error
- [x] Sabotaje: eliminar índice de slug → create duplicado pasa (unique constraint lo evita)

## Cierre

- [x] Verificación manual: crear REQ desde CLI, verificar en DB, listar, archivar
- [x] Suite verde
