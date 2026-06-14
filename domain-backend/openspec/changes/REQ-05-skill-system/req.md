# REQ-05-skill-system: Sistema de skills: definiciones reutilizables (prompt/code/API/MCP tool), registro con búsqueda semántica, versionado, dependencias, auto-skill matching.

**Estado:** activo
**Creado:** 2026-06-07
**Fase:** F2

## Descripción

Sistema de skills: definiciones reutilizables (prompt/code/API/MCP tool), registro con búsqueda semántica, versionado, dependencias, auto-skill matching.

## Criterios de éxito

- Skills CRUD funcional con 4 tipos (prompt, code, api, mcp_tool)
- Búsqueda full-text y semántica operativa
- Versionado completo con pin, rollback y diff
- Auto-recomendación de skills por contexto
- Ejecución de skills con timeout y logging
- Contrato Agent↔Skill formalizado: JSON Schema input/output, tool-calling format por provider, taxonomía de errores tipados, idempotency hints, skill-to-skill invocation con chain depth limit

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| issue-05.1-skill-definitions | proposed | Skill CRUD: name, slug, description, type, content, parameters schema, return type, tags, embedding |
| issue-05.2-skill-registry-search | proposed | Central registry con FTS (tsvector), búsqueda semántica (pgvector), filtros por type/project/tags |
| issue-05.3-skill-versioning | proposed | Versionado: cada update crea versión, pin a versión, rollback, diff, breaking changes |
| issue-05.4-auto-skill-engine | proposed | Auto-recomendación: embed contexto → similarity search → top-N con relevance scores |
| issue-05.5-skill-execution | proposed | Ejecución sync/async: resuelve versión, construye contexto, captura output, log, timeout |
| issue-05.6-agent-skill-contract | proposed | Contrato Agent↔Skill: JSON Schema validation, tool-calling translation 4 providers, error taxonomy, idempotency, timeout, skill chain |
