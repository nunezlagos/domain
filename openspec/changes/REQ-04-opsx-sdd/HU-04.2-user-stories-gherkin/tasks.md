# Tasks: HU-04.2-user-stories-gherkin

## Backend

- [ ] `migrations/XXXX_create_user_stories.sql`: tabla + índices + FK a requirements
- [ ] `migrations/XXXX_create_gherkin_scenarios.sql`: tabla + FK a user_stories + position
- [ ] `internal/opsx/user_story.go`: structs `UserStory`, `GherkinScenario`, `UserStoryFilter`
- [ ] `internal/store/pg/user_story.go`: interfaz `UserStoryStore`
- [ ] Implementar `Create(story UserStory, scenarios []GherkinScenario) (uuid.UUID, error)`
- [ ] Implementar `GetBySlug(slug string) (*UserStory, error)`
- [ ] Implementar `GetByID(id uuid.UUID) (*UserStory, error)`
- [ ] Implementar `Update(slug string, update UserStoryUpdate) error`
- [ ] Implementar `List(filter UserStoryFilter) ([]UserStory, int, error)`
- [ ] Implementar `Delete(slug string) error`
- [ ] Implementar `AddScenario(huSlug string, scenario GherkinScenario) error`
- [ ] Implementar `RemoveScenario(scenarioID uuid.UUID) error`
- [ ] Implementar `UpdateScenario(scenarioID uuid.UUID, scenario GherkinScenario) error`
- [ ] `internal/opsx/user_story_service.go`: validaciones (slug, given/then not empty, REQ exists)

## Tests

- [ ] Test unitario: validación de formato slug HU
- [ ] Test unitario: validación de Gherkin (given/then no vacío)
- [ ] Test de integración: crear HU con escenarios → consultar → verificar structured data
- [ ] Test de integración: agregar/eliminar escenarios
- [ ] Test de integración: filtrar por req_slug y status
- [ ] Test de slug duplicado → error
- [ ] Test de FK: eliminar REQ con HUs → error ON DELETE RESTRICT
- [ ] Sabotaje: dropear índice de slug → unique constraint lo atrapa

## Cierre

- [ ] Verificación manual: crear HU con escenarios vía API, verificar DB
- [ ] Suite verde
