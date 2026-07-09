# Changelog

Todos los cambios notables de domain-mcp se documentan en este archivo.

El formato sigue [Keep a Changelog](https://keepachangelog.com/es-ES/1.0.0/).

## [Unreleased]

### Corregido

- **SDD step delegation contract — R5 (issue-64.2)**: contrato de delegación completo entre el orquestador y el cliente que ejecuta las fases.
  - **R5-A — contrato upfront**: `PhaseStepSummary` ahora expone `required_tool_calls` y `output_schema` (JSON Schema) de cada fase, poblados desde su definición y exportados en `exportPlan`. El cliente conoce el contrato antes de ejecutar, sin descubrirlo por rechazo. `sdd-spec` declara su `output_schema` (`issue_slug`, `issue_md`).
  - **R5-B — validación agregada**: al reportar una fase, la validación evalúa en una sola pasada los `required_saves`, el shape del output y los `tool_calls` faltantes, y los devuelve TODOS juntos, en vez de rechazar de a uno. `sdd-spec.Validate` acumula sus campos faltantes. El step sigue quedando running/reintentable.

- **SDD pipeline hardening (issue-64.1)**: tres contratos del pipeline SDD, verificados contra el código (recomendaciones R3/R2/R7 de la guía fable-5).
  - **R3 — formato de escenarios**: `ParseScenarios` ahora acepta heading `##` y `####`, y `Given/When/Then` plano, con bullet o con negrita (`- **Given**`). La policy `openspec-spec-format` y el prompt `sdd-spec` documentan el formato canónico y aclaran las variantes toleradas. El error de spec sin escenarios incluye un ejemplo mínimo.
  - **R2 — round-trip de tasks**: `sdd-tasks` devuelve `created_task_ids` en `PhaseResultResult` (antes descartaba los IDs generados por `CreateTasks`), y `applyTasks` reporta el conteo de tasks sin marcador ignoradas en `ApplyResult.ignored_tasks`.
  - **R7 — errores de apply**: `ApplyResult` distingue `not_sent` (archivo omitido del array), `unknown_issue` (issue no está en BD, con hint accionable) y `conflict` (hash divergente). El mensaje genérico de `issue_id inválido` incluye ahora una guía de resolución.

### Añadido

- **Memory graph** (mig 000175): grafo de relaciones explícitas y tipadas entre `knowledge_observations`, con aristas bi-temporales (valid_from/valid_to para valid time, created_at para transaction time). Tipos dirigidos: supersedes, derived_from, depends_on, contradicts, relates_to. Tabla `knowledge_observation_edges` + `observation/edge.go` + `memory_graph_tools.go`.
- **Code graph** (mig 000176): grafo de código del repo (Go v1) persistido en Postgres. Tablas `code_nodes`, `code_edges`, `code_index_files` con incrementalidad por content_hash + git_head. Paquete `internal/service/codegraph` + `code_graph_tools.go`.
- **Cruce memoria <-> código** (mig 000177): vínculo dirigido observation -> code_node (affects, decided_in, references, implements) que conecta el memory graph con el code graph. Tabla `knowledge_observation_code_links` + `codegraph/link.go`.
