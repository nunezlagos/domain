# Tasks: issue-08.1-agent-definitions

> Nota de schema: agents guarda skills como `skills_slugs TEXT[]` (validados
> en service contra la tabla skills) en lugar de tabla relacional
> `agent_skills` — decisión original de 000012, <50 skills por agent.
> `max_tokens` per-call no existe en el schema; el guardrail equivalente es
> `token_budget` (presupuesto total del run, issue-07.4 lo clampa al
> context_size del registry en runtime).

## Backend

- [x] Migración SQL → 000012 agents + 000083 agent_versions (snapshot JSONB, UNIQUE agent_id+version) — 2026-06-11
- [x] Modelo `Agent` con todos los campos → internal/service/agent (provider, model, system_prompt, skills_slugs, guardrails)
- [x] CRUD → Create/GetByID/GetBySlug/List/Update/SoftDelete sobre pgx (sin capa repository separada: service usa Pool directo, patrón del proyecto)
- [x] Validación: model exists en model_registry (ollama exento: modelos locales arbitrarios), skills exist, temperatura [0,2] → ErrModelUnknown/ErrSkillNotFound/ErrTemperatureRange — 2026-06-11
- [x] Slug generator → slugify(name) + unicidad con sufijo -2..-N cuando Slug viene vacío — 2026-06-11
- [x] Versionado automático → Update archiva snapshot previo en agent_versions (archiveVersion) — 2026-06-11
- [x] Endpoints HTTP → POST/GET /agents, GET/PATCH/DELETE /agents/{id} (PATCH en lugar de PUT, convención api.md)
- [x] Asignación de skills en create/update → skills_slugs validados en ambos
- [x] Límite de versiones (50) con purge automático → DELETE version <= MAX-50 en cada archive — 2026-06-11
- [x] GET /agents/{id}/versions → listAgentVersions (más reciente primero, anti-enumeration 404) — 2026-06-11

## Tests

- [x] Creación válida → TestAgent_Create (slug, provider, defaults)
- [x] Campos requeridos → ErrNameRequired/ErrModelRequired/ErrProviderInvalid (TestAgent_Create_InvalidProvider)
- [x] Slug único + colisión genera slug-N → TestAgent_Create_AutoSlug_CollisionSuffix — 2026-06-11
- [x] Temperatura fuera de [0,2] rechazada → TestAgent_Create_TemperatureOutOfRange (2.5 y -0.1) — 2026-06-11
- [x] max_tokens vs model → N/A por schema (ver nota); token_budget clamp testeado en issue-07.4
- [x] Asignación de skills → TestAgent_Create_WithValidSkills + RejectsUnknownSkills
- [x] Versionado N updates = N versions → TestAgent_Update_Versioning (snapshot guarda config PREVIA) — 2026-06-11
- [x] Purge a 50 → TestAgent_Versions_PurgeOver50 (55 + 1 → quedan 50, viejas purgadas) — 2026-06-11
- [x] Integración CRUD con DB real → suite testcontainers completa (15 tests)
- [x] E2E vía API HTTP → api_integration_test.go cubre handlers agents
- [x] Sabotaje: model inexistente → TestSabotage_Agent_UnknownModelRejected (+ ollama exento) — 2026-06-11
- [x] Sabotaje: skill fantasma en Update → TestSabotage_Agent_UpdateRejectsBadSkill

## Cierre

- [x] Verificación CRUD → cubierta por integración HTTP (mismo código de producción)
- [x] Suite verde completa → 2026-06-11 (1037 short + 15 integration agent)
- [x] Documentar endpoints y fields → snapshots API (testdata/api/) regenerados con /versions
