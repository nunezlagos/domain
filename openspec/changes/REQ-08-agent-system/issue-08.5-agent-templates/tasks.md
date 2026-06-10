# Tasks: issue-08.5-agent-templates

## Backend

- [ ] Crear migración SQL para tablas `agent_templates` y `agent_template_skills`
- [ ] Seedear 4 templates: Code Reviewer, Architecture Advisor, Bug Hunter, PR Reviewer
- [ ] Implementar `AgentTemplateRepository` con GetAll y GetBySlug
- [ ] Implementar `AgentTemplateService` con métodos GetAll, GetBySlug, Instantiate
- [ ] Implementar `Instantiate()`: toma slug + overrides → crea Agent via AgentService.Create
- [ ] Implementar validación de overrides (temperature range, model exists, skills exist)
- [ ] Implementar endpoint GET /agent-templates
- [ ] Implementar endpoint POST /agents/from-template con body slug + overrides
- [ ] Definir contenido de cada template: system prompt detallado, skills recomendados, temp, modelo
- [ ] Code Reviewer: temp=0.3, skills=[read-file, list-files, search-code], model=gpt-4
- [ ] Architecture Advisor: temp=0.5, skills=[analyze-diagram, list-files], model=claude-3
- [ ] Bug Hunter: temp=0.2, skills=[read-file, search-code, git-log], model=gpt-4
- [ ] PR Reviewer: temp=0.3, skills=[read-file, git-diff, list-files], model=gpt-4

## Tests

- [ ] Test unitario: GetAll retorna 4 templates
- [ ] Test unitario: GetBySlug existente retorna template
- [ ] Test unitario: GetBySlug inexistente retorna error
- [ ] Test unitario: Instantiate con overrides válidos crea agente correctamente
- [ ] Test unitario: Instantiate con override inválido (temp=5) retorna 422
- [ ] Test unitario: Instantiate sin overrides usa defaults del template
- [ ] Test de integración: template → agent creation pipeline completo
- [ ] Test E2E: escenarios Gherkin del hu.md
- [ ] Sabotaje: template no encontrado → 404

## Cierre

- [ ] Verificación manual: POST /agents/from-template con cada template
- [ ] Suite verde completa
- [ ] Documentar templates disponibles y sus configuraciones
