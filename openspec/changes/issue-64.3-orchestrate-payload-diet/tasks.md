# issue-64.3 — Tasks

## Implementación

- [ ] hydrateSystemPrompts (service.go): en modo full, hidratar SystemPrompt solo del step 0
- [ ] confirmar que el SystemPrompt de todos los steps se sigue persistiendo en step.Inputs (repository.go)
- [ ] agregar campo NextStepSystemPrompt a PhaseResultResult (phase_result.go)
- [ ] phase_result: poblar NextStepSystemPrompt leyendo step.Inputs["system_prompt"] del siguiente step
- [ ] mantener NextStepPrompt (user) sin cambios (retrocompat)

## Tests

- [ ] exportPlan/hydrate deja SystemPrompt vacío en steps 2..N (full)
- [ ] step 0 conserva SystemPrompt hidratado
- [ ] SystemPrompt persistido en step.Inputs de steps 2..N
- [ ] phase_result devuelve NextStepSystemPrompt no vacío del siguiente step
- [ ] NextStepPrompt (user) sigue presente
- [ ] Sabotaje: hydrate eager de todos → test de "vacío en 2..N" falla

## Documentación

- [ ] Actualizar CHANGELOG Unreleased
- [ ] Actualizar state.yaml a implemented al cerrar
