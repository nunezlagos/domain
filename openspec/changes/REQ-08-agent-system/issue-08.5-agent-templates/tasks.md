# Tasks: issue-08.5-agent-templates

## Backend

- [x] Crear migración SQL para tablas `agent_templates` y `agent_template_skills`
- [x] Seedear 4 templates: Code Reviewer, Architecture Advisor, Bug Hunter, PR Reviewer
- [x] Implementar `AgentTemplateRepository` con GetAll y GetBySlug
- [x] Implementar `AgentTemplateService` con métodos GetAll, GetBySlug, Instantiate
- [x] Implementar `Instantiate()`: toma slug + overrides → crea Agent via AgentService.Create
- [x] Implementar validación de overrides (temperature range, model exists, skills exist)
- [x] Implementar endpoint GET /agent-templates
- [x] Implementar endpoint POST /agents/from-template con body slug + overrides
- [x] Definir contenido de cada template: system prompt detallado, skills recomendados, temp, modelo
- [x] Code Reviewer: temp=0.3, skills=[read-file, list-files, search-code], model=gpt-4
- [x] Architecture Advisor: temp=0.5, skills=[analyze-diagram, list-files], model=claude-3
- [x] Bug Hunter: temp=0.2, skills=[read-file, search-code, git-log], model=gpt-4
- [x] PR Reviewer: temp=0.3, skills=[read-file, git-diff, list-files], model=gpt-4

## Tests

- [x] Test unitario: GetAll retorna 4 templates
- [x] Test unitario: GetBySlug existente retorna template
- [x] Test unitario: GetBySlug inexistente retorna error
- [x] Test unitario: Instantiate con overrides válidos crea agente correctamente
- [x] Test unitario: Instantiate con override inválido (temp=5) retorna 422
- [x] Test unitario: Instantiate sin overrides usa defaults del template
- [x] Test de integración: template → agent creation pipeline completo
- [x] Test E2E: escenarios Gherkin del hu.md
- [x] Sabotaje: template no encontrado → 404

## Cierre

- [x] Verificación manual: POST /agents/from-template con cada template
- [x] Suite verde completa
- [x] Documentar templates disponibles y sus configuraciones
