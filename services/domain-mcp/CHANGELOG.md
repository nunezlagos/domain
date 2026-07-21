# Changelog

Todos los cambios notables de domain-mcp se documentan en este archivo.

El formato sigue [Keep a Changelog](https://keepachangelog.com/es-ES/1.0.0/).

## [Unreleased]

### Corregido

- **Rotación y dedup de backups del installer (issue-65.2)**: el installer acumulaba backups sin límite (332 en la máquina del usuario, ~192 de un solo `domain-login.md`). Dos causas: `InstallSlashCommand` renombraba el archivo a `.backup-<ts>` de forma INCONDICIONAL en cada corrida (la dispara el hook SessionStart en cada sesión), y el wrapper `BackupFile` invocaba `backupFile(path, 0)` desactivando la poda que `backupFile` ya soportaba. Fix: `BackupFile` pasa `keepLast=3`; `InstallSlashCommand` compara contenido y hace early-return si es idéntico, y si difiere usa `install.BackupFile` (dedup por SHA-256 + poda, convención `.bak.<ts>`) en vez de `os.Rename`. En el módulo `install-user` (go.mod propio, no puede importar el server) `backupIfExists` reimplementa dedup por hash + poda `keepLast=3` y `writeEnvIfConfigured` pasa por él. Se preserva la convención `.bak.<ts>` del server porque `isBackupPath`/`Restore` la validan estrictamente; NO se unifica a `.backup-<ts>` (rompería el restore). Call sites relacionados (`mcpinstaller/installer.go`, `writeJSONWithBackup`) quedan como follow-up en DOMAINSERV-2.

- **Sincronización tool→canal (issue-54.1)**: `domain_code_graph` pasa de `ChannelHook` a `ChannelUserIntent` (fue deprecada en el kill 007 del 2026-07-07 y ya no se pre-carga por hook; la etiqueta declaraba un comportamiento inexistente). `TOOL_CHANNELS.md` se regeneró consistente con el map `toolChannel` (estaba desincronizado: faltaban `domain_issue_list` y `domain_flow_cancel`, y `domain_openspec_apply`/`_export` figuraban en el canal equivocado). Se agregó `TestToolChannelsDocInSync` que valida la sincronización doc↔map bidireccional, para que CI detecte futuras divergencias. El canal es metadata de auditoría + contrato de CI, no afecta runtime.

- **SDD orchestrate payload diet — R4 (issue-64.3)**: `domain_orchestrate` en modo full ya no embebe el `SystemPrompt` (template + ~20 policies) de los 11 steps en el payload inicial. `exportPlan` omite el `SystemPrompt` de los steps 2..N (solo el step 0 lo lleva), bajando el payload de ~220-320KB a ~20-40KB. El `SystemPrompt` de cada step sigue persistido en `step.Inputs` y se entrega reconstruido en `PhaseResultResult.next_step_system_prompt` al cerrar cada fase. Campo aditivo; `NextStepPrompt` (user) sin cambios. Modos express/lite conservan todo, y el modo detect (preview no persistido) también conserva el SystemPrompt (no hay `step.Inputs` de dónde reconstruirlo).

- **SDD step delegation contract — R5 (issue-64.2)**: contrato de delegación completo entre el orquestador y el cliente que ejecuta las fases.
  - **R5-A — contrato upfront**: `PhaseStepSummary` ahora expone `required_tool_calls` y `output_schema` (JSON Schema) de cada fase, poblados desde su definición y exportados en `exportPlan`. El cliente conoce el contrato antes de ejecutar, sin descubrirlo por rechazo. `sdd-spec` declara su `output_schema` (`issue_slug`, `issue_md`).
  - **R5-B — validación agregada**: al reportar una fase, la validación evalúa en una sola pasada los `required_saves`, el shape del output y los `tool_calls` faltantes, y los devuelve TODOS juntos, en vez de rechazar de a uno. `sdd-spec.Validate` acumula sus campos faltantes. El step sigue quedando running/reintentable.

- **SDD pipeline hardening (issue-64.1)**: tres contratos del pipeline SDD, verificados contra el código (recomendaciones R3/R2/R7 de la guía fable-5).
  - **R3 — formato de escenarios**: `ParseScenarios` ahora acepta heading `##` y `####`, y `Given/When/Then` plano, con bullet o con negrita (`- **Given**`). La policy `openspec-spec-format` y el prompt `sdd-spec` documentan el formato canónico y aclaran las variantes toleradas. El error de spec sin escenarios incluye un ejemplo mínimo.
  - **R2 — round-trip de tasks**: `sdd-tasks` devuelve `created_task_ids` en `PhaseResultResult` (antes descartaba los IDs generados por `CreateTasks`), y `applyTasks` reporta el conteo de tasks sin marcador ignoradas en `ApplyResult.ignored_tasks`.
  - **R7 — errores de apply**: `ApplyResult` distingue `not_sent` (archivo omitido del array), `unknown_issue` (issue no está en BD, con hint accionable) y `conflict` (hash divergente). El mensaje genérico de `issue_id inválido` incluye ahora una guía de resolución.

### Añadido

- **Provider LLM OpenAI-compatible configurable (DOMAINSERV-56)**: nuevo `openai.NewWithBaseURL(apiKey, baseURL, model)` + `openai.RegisterOpenAICompat(factory, wrap, logger)` que registra 1..N endpoints OpenAI-compatibles (vLLM/Groq/Together/LM Studio) desde `DOMAIN_OPENAI_COMPAT_PROVIDERS` (JSON array `{name, base_url, api_key_env, model}`), reusando el dialecto openai con `base_url` override. Espejo del patrón `anthropic/minimax.go`. El `api_key` se referencia por nombre de env (`api_key_env`), nunca el valor, y no se loguea. Parse tolerante: item inválido → warning (sin key) + skip, sin abortar el boot. Cableado en `buildLLMFactory` junto a los providers existentes (sin tocarlos). Base del epic DOMAINSERV-55.
- **Memory graph** (mig 000175): grafo de relaciones explícitas y tipadas entre `knowledge_observations`, con aristas bi-temporales (valid_from/valid_to para valid time, created_at para transaction time). Tipos dirigidos: supersedes, derived_from, depends_on, contradicts, relates_to. Tabla `knowledge_observation_edges` + `observation/edge.go` + `memory_graph_tools.go`.
- **Code graph** (mig 000176): grafo de código del repo (Go v1) persistido en Postgres. Tablas `code_nodes`, `code_edges`, `code_index_files` con incrementalidad por content_hash + git_head. Paquete `internal/service/codegraph` + `code_graph_tools.go`.
- **Cruce memoria <-> código** (mig 000177): vínculo dirigido observation -> code_node (affects, decided_in, references, implements) que conecta el memory graph con el code graph. Tabla `knowledge_observation_code_links` + `codegraph/link.go`.
