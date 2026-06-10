# Domain — Cloud-First Agent Platform

Stack: Go 1.22+ · pgx v5 · Postgres 15+ (pgvector, tsvector)
Arquitectura: Single Go binary, cloud-first, MCP bidireccional

## SDD Workflow (openspec/changes/)

Cada feature sigue: REQ → HU → Proposal → Design → Tasks → TDD → Sabotaje

- `openspec/changes/REQ-XX-slug/` — requisitos activos
- `openspec/changes/archive/` — requisitos archivados
- Cada HU tiene 5 archivos: `issue.md` (Gherkin), `proposal.md`, `design.md` (ADR+TDD), `tasks.md`, `state.yaml`

Regla: NO implementar nada sin una HU que lo respalde. Si falta una HU: crearla con opsx.

## Skills

- Skills reutilizables: tipo prompt, code, API, MCP tool
- Auto-skill: matching semántico por pgvector entre query y skills registrados
- Versionado semántico (major.minor.patch) con changelog por versión
- Registry search por nombre, tipo, descripción, embedding similarity
- Skills pueden depender de otros skills y de MCPs externos (issue-12.4)

## Flows

- DAG de pasos: skill_call, llm_call, code_exec, conditional, parallel, wait, human_input, agent_run, sub_flow, transform
- State machine: pending → running → completed/failed/paused/cancelled
- Retry policies: backoff exponencial, ignore/abort/fallback, Dead Letter Queue
- Sub-flows con contexto padre→hijo y detección de circularidad

## Agentes

- Definiciones: modelo, provider, system prompt, skills asignados, temperatura
- Ejecución con sesión: run + logs + tokens + costo
- Multi-agente: supervisor delega a subagentes con handoff y paralelismo
- Templates predefinidos: Code Reviewer, Architecture Advisor, Bug Hunter, PR Reviewer

## Proyectos

- Cada proyecto identificado por UUID + repository_url (git remote)
- Templates de proyecto definen skills default, scope de memoria, agentes preconfigurados
- Merge entre proyectos con resolución de conflictos (rename + dedup)
- Cross-project references (read-only) entre proyectos linkeados

## Arquitectura clave

- Postgres-only: nada de SQLite. pgvector para embeddings, tsvector para FTS
- LLM Provider factory: OpenAI, Anthropic, Google, Ollama
- MCP tools con prefijo `domain_` (coexisten con tools nativas del agente)
- API key auth resuelve a user_id + organization_id
- Observaciones tienen created_by FK → users, project_id FK → projects
- Activity log general (issue-02.6) + Audit log de seguridad (issue-02.4)
- S3 storage para adjuntos de opsx, knowledge docs y skills
- Runners: sandbox Docker cloud + self-hosted WebSocket

## Clean Architecture (ver .claude/rules/clean-architecture.md)

Domain sigue Clean Architecture organizada por features, no por capas técnicas.
Cada feature tiene su propia entidad, repository interface, service, store y handlers.
Dependency Rule: las dependencias apuntan hacia adentro (domain ← service ← store/mcp/api).

## Convenciones Go

- `internal/store/pg/` — repositorios con interfaces
- `internal/service/` — lógica de negocio
- `internal/mcp/tools/` — handlers MCP
- `cmd/domain/` — CLI entrypoints
- `migrations/` — golang-migrate SQL plano
- Tests de integración con Postgres real (testcontainers)
- Logger estructurado (slog), errores con stack (fmt.Errorf + %w)

## TDD (Red → Green → Refactor → Sabotaje)

1. Red: escribir test que falla
2. Green: implementación mínima
3. Refactor: mejorar sin romper tests
4. Sabotaje: romper fix → test cae → restaurar

## Estado actual

16 REQs, 73 HUs, ~400 archivos de spec activos (más 361 archivados del legacy). Ver `openspec/changes/` para detalle.
