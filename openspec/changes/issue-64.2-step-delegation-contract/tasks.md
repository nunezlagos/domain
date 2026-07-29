# issue-64.2 — Tasks

## Implementación

- [ ] A: agregar RequiredToolCalls []string y OutputSchema map[string]any a PhaseStepSummary (types.go)
- [ ] A: definir el contrato (required_tool_calls + output_schema) por fase en phases/registry.go
- [ ] A: poblar los campos nuevos en exportPlan (service.go)
- [ ] B: refactor de la validación de phase_result para acumular en una sola pasada
- [ ] B: correr output-fields + required_saves + tool_calls sin cortar en el primero
- [ ] B: devolver las 3 categorías juntas (conservar validation_error string agregado)

## Tests

- [ ] A: exportPlan incluye required_tool_calls de la fase
- [ ] A: exportPlan incluye output_schema de la fase
- [ ] A: fase sin contrato no rompe el plan (campos omitidos)
- [ ] B: múltiples campos faltantes se reportan juntos
- [ ] B: faltantes de distintas categorías juntos
- [ ] B: step sigue running tras rechazo agregado
- [ ] Sabotaje: acumulador que corta al primer error → test de "juntos" falla

## Documentación

- [ ] Actualizar CHANGELOG Unreleased
- [ ] Bumpear Version() de seeders si se tocan prompts/policies (gotcha seeder)
- [ ] Actualizar state.yaml a implemented al cerrar
