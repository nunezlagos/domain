# Proposal: HU-08.1-agent-definitions

## Intención

Implementar CRUD completo de definiciones de agente con persistencia, validación de campos, asignación de skills, slug único, y versionado.

## Scope

**In scope:**
- Modelo `Agent` con campos: ID, Name, Slug, Description, ModelID, SystemPrompt, Temperature, MaxTokens, ProjectID, CreatedAt, UpdatedAt, Version
- Tablas: `agents`, `agent_skills`, `agent_versions`
- CRUD endpoints: POST/GET /agents, GET/PUT/DELETE /agents/:id
- Slug auto-generado desde name con unicidad por project_id
- Asignación de skills vía lista de skill slugs
- Versionado: cada update crea un snapshot en `agent_versions`
- Validación: modelo existe en registry, skills existen, temperatura [0-2], max_tokens <= model.max_tokens

**Out of scope:**
- Agent templates (eso es HU-08.5)
- Agent execution (eso es HU-08.2)
- Agentes globales sin project_id

## Enfoque técnico

- Tabla `agents` con FK a `models` y `projects`
- Tabla `agent_skills` con FK a `agents` y `skills`
- Tabla `agent_versions` con snapshot JSON del agente + diff
- Slug: `slugify(name)` + verificación de unicidad (si existe, append -N)
- CRUD vía HTTP API con handlers standard
- Validación en service layer antes de persistir

## Riesgos

- Slug único puede colisionar en writes concurrentes → usar unique constraint + retry
- Skills referencian slugs que pueden cambiar → guardar skill_id (FK) no slug
- Versionado puede crecer rápido → límite de versiones (50) con purge automático

## Testing

- **Unit:** Agent validation, slug generation, version creation
- **Integration:** CRUD con DB real, skill assignment, version history
- **Gherkin:** Escenarios del hu.md
- **Sabotaje:** Crear agente con model inexistente → debe fallar
