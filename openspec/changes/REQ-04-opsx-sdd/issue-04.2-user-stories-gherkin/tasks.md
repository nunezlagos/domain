# Tasks: issue-04.2-user-stories-gherkin

## Backend

- [x] `migrations/XXXX_create_issues.sql`: tabla + índices + FK a requirements
- [x] `migrations/XXXX_create_gherkin_scenarios.sql`: tabla + FK a issues + position
- [x] `internal/opsx/user_story.go`: structs `Issue`, `GherkinScenario`, `UserStoryFilter`
- [x] `internal/store/pg/user_story.go`: interfaz `UserStoryStore`
- [x] Implementar `Create(story Issue, scenarios []GherkinScenario) (uuid.UUID, error)`
- [x] Implementar `GetBySlug(slug string) (*Issue, error)`
- [x] Implementar `GetByID(id uuid.UUID) (*Issue, error)`
- [x] Implementar `Update(slug string, update UserStoryUpdate) error`
- [x] Implementar `List(filter UserStoryFilter) ([]Issue, int, error)`
- [x] Implementar `Delete(slug string) error`
- [x] Implementar `AddScenario(huSlug string, scenario GherkinScenario) error`
- [x] Implementar `RemoveScenario(scenarioID uuid.UUID) error`
- [x] Implementar `UpdateScenario(scenarioID uuid.UUID, scenario GherkinScenario) error`
- [x] `internal/opsx/user_story_service.go`: validaciones (slug, given/then not empty, REQ exists)

## Tests

- [x] Test unitario: validación de formato slug HU
- [x] Test unitario: validación de Gherkin (given/then no vacío)
- [x] Test de integración: crear HU con escenarios → consultar → verificar structured data
- [x] Test de integración: agregar/eliminar escenarios
- [x] Test de integración: filtrar por req_slug y status
- [x] Test de slug duplicado → error
- [x] Test de FK: eliminar REQ con HUs → error ON DELETE RESTRICT
- [x] Sabotaje: dropear índice de slug → unique constraint lo atrapa

## Cierre

- [x] Verificación manual: crear HU con escenarios vía API, verificar DB
- [x] Suite verde
