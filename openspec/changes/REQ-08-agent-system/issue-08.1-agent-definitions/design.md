# Design: issue-08.1-agent-definitions

## Decisión arquitectónica

**Patrón:** Repository + Service + Handler (3-layer).

```
HTTP Handler → AgentService → AgentRepository → DB
                  │
                  ▼
            Validation:
            - ModelExists(modelID)
            - SkillsExist(skillIDs)
            - Temperature [0-2]
            - MaxTokens ≤ model.MaxTokens
```

- `AgentService` contiene toda la lógica de negocio (validación, slug generation, versionado)
- `AgentRepository` acceso a datos (CRUD, versions, skills)
- Versionado es automático en service: cada `Update()` crea un `AgentVersion`
- Skills se almacenan como IDs (FK a `skills`) en tabla puente `agent_skills`

## Alternativas descartadas

1. **JSONB para agent_versions (todo en un campo):** Dificulta query de versiones específicas. Preferimos fila por versión con snapshot JSON + diff.
2. **Soft delete:** Preferimos DELETE real con archivo en agent_versions (última versión = deleted).
3. **Skills como JSON array en agents:** Rompe normalización. Tabla puente permite query eficiente "qué agentes usan skill X".

## Diagrama

```
Tablas:
┌───────────────┐     ┌──────────────┐     ┌────────────────┐
│    agents     │     │ agent_skills │     │ agent_versions │
├───────────────┤     ├──────────────┤     ├────────────────┤
│ id (PK)       │────▶│ agent_id (FK)│     │ id (PK)        │
│ project_id(FK)│     │ skill_id (FK)│     │ agent_id (FK)  │
│ name          │     └──────────────┘     │ version        │
│ slug (UQ)     │                           │ snapshot (JSONB)│
│ description   │                           │ diff (JSONB)   │
│ model_id (FK) │                           │ created_at     │
│ system_prompt │                           └────────────────┘
│ temperature   │
│ max_tokens    │
│ is_active     │
│ created_at    │
│ updated_at    │
└───────────────┘
```

## TDD plan

1. **Red:** Test creación de agente con datos válidos retorna agente con slug y version=v1
2. **Green:** Implementar AgentService.Create() + AgentRepository
3. **Refactor:** Extraer slug generator, validators
4. **Sabotaje:** ModelID inválido → Create() debe retornar error

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|-----------|
| Race condition en slug único | Unique constraint en slug + retry con slug-N |
| Versionado crece indefinido | Límite de 50 versiones por agente; purge de las más viejas |
| Skills referenciados se eliminan | Validar existencia en Create/Update; nullable skill_id no elimina agente (solo desasigna) |
| Proyecto no existe | FK constraint + validación en service
