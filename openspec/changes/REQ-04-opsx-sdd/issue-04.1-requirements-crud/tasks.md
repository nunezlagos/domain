# Tasks: issue-04.1-requirements-crud

## Backend

- [ ] `migrations/XXXX_create_requirements.sql`: tabla + índices + self-FK
- [ ] `internal/opsx/requirement.go`: structs `Requirement`, `RequirementFilter`, `RequirementTree`
- [ ] `internal/store/pg/requirement.go`: interfaz `RequirementStore`
- [ ] Implementar `Create(req Requirement) (uuid.UUID, error)` con validación de slug y parent
- [ ] Implementar `GetBySlug(slug string) (*Requirement, error)`
- [ ] Implementar `GetByID(id uuid.UUID) (*Requirement, error)`
- [ ] Implementar `Update(slug string, req RequirementUpdate) error`
- [ ] Implementar `Archive(slug string, recursive bool) error`
- [ ] Implementar `List(filter RequirementFilter) ([]Requirement, int, error)`
- [ ] Implementar `GetTree(slug string) (*RequirementTree, error)` con CTE recursiva
- [ ] `internal/opsx/requirement_service.go`: lógica de negocio, validaciones

## Tests

- [ ] Test unitario de validación de slug (formato correcto/incorrecto)
- [ ] Test de integración: crear raíz + hijo + árbol
- [ ] Test de integración: filtros combinados (status + priority)
- [ ] Test de integración: archive sin cascade → hijos no se archivan
- [ ] Test de integración: archive con cascade → todos archivados
- [ ] Test de slug duplicado → error
- [ ] Test de parent_id inexistente → error
- [ ] Sabotaje: eliminar índice de slug → create duplicado pasa (unique constraint lo evita)

## Cierre

- [ ] Verificación manual: crear REQ desde CLI, verificar en DB, listar, archivar
- [ ] Suite verde
