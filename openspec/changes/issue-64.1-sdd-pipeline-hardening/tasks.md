# issue-64.1 — Tasks

## Implementación

- [ ] R3: ampliar `ParseScenarios` (parse.go) para aceptar `## ` y `#### ` en el heading de Scenario
- [ ] R3: aceptar Given/When/Then plano, con bullet `- ` y con bold `- **...**`
- [ ] R3: alinear texto de policy `openspec-spec-format` (platform_policies_seeder.go)
- [ ] R3: alinear prompt de sdd-spec (agent_templates_catalog.go)
- [ ] R3: enriquecer el error "spec.md no contiene escenarios válidos" (engine.go) con un ejemplo mínimo
- [ ] R2: capturar los IDs de `CreateTasks` en persistTasksOpenspec (phase_result.go)
- [ ] R2: agregar campo `created_task_ids` a `PhaseResultResult` y poblarlo
- [ ] R2: `applyTasks` cuenta tasks sin marcador y lo reporta en `ApplyResult`
- [ ] R7: extender `ApplyResult` con clasificación not_sent / unknown_issue / conflict
- [ ] R7: `applyFiles` clasifica archivos omitidos como not_sent (no conflict)
- [ ] R7: reemplazar mensaje genérico "issue_id inválido o falta .openspec.yaml" por uno específico con hint

## Tests

- [ ] Test R3: H4+bullets bold parsea (red→green)
- [ ] Test R3: H2+plano parsea
- [ ] Test R3: error de spec vacío incluye ejemplo
- [ ] Test R2: handler devuelve created_task_ids
- [ ] Test R2: apply reporta tasks ignoradas sin marcador
- [ ] Test R7: archivo omitido → not_sent, no conflict
- [ ] Test R7: issue inexistente → unknown_issue con hint
- [ ] Test R7: hash divergente → conflict
- [ ] Test sabotaje: romper una variante del parser → verificar que su test falla

## Documentación

- [ ] Actualizar CHANGELOG Unreleased
- [ ] Actualizar state.yaml a implemented al cerrar
