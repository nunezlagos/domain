# Tasks: issue-08.1-agent-definitions

## Backend

- [ ] Crear migración SQL para tablas `agents`, `agent_skills`, `agent_versions`
- [ ] Implementar modelo `Agent` con todos los campos definidos
- [ ] Implementar `AgentRepository` con métodos CRUD
- [ ] Implementar `AgentService` con validación: model exists, skills exist, temp range, max_tokens ≤ model
- [ ] Implementar slug generator: slugify(name) + unicidad por proyecto
- [ ] Implementar versionado automático: cada Update crea AgentVersion con snapshot
- [ ] Implementar endpoints HTTP: POST/GET /agents, GET/PUT/DELETE /agents/:id
- [ ] Implementar asignación de skills en create/update
- [ ] Implementar límite de versiones (50) con purge automático
- [ ] Exponer GET /agents/:id/versions para historial

## Tests

- [ ] Test unitario: creación agente válido retorna slug + version v1
- [ ] Test unitario: validación de campos requeridos (name, model)
- [ ] Test unitario: slug único (colisión genera slug-N)
- [ ] Test unitario: temperatura fuera de rango [0-2] rechazada
- [ ] Test unitario: max_tokens > model.max_tokens rechazado
- [ ] Test unitario: asignación de skills
- [ ] Test unitario: versionado (N updates = N versions)
- [ ] Test de integración: CRUD completo con DB real
- [ ] Test E2E: escenarios Gherkin del hu.md vía API HTTP
- [ ] Sabotaje: model inexistente → Create falla

## Cierre

- [ ] Verificación manual: curl POST/GET/PUT/DELETE agentes
- [ ] Suite verde completa
- [ ] Documentar endpoints y fields
