# Tasks: issue-42.3-drop-legacy-infra

> **Orden de deploy:** CODE-FIRST. Remover/refactorizar TODO el código (secciones de abajo) → `go build`/`go vet`/`go test` verdes → recién ahí aplicar la migration 000149. Nunca dropear la tabla antes que el código (el binario corriendo crashea al boot/request).

## Verificación previa (bloqueante)

- [ ] Confirmar las 8 tablas con 0 filas en el VPS (introspección: confirmado)
- [ ] Decisiones resueltas: `sabotage_records` se PRESERVA (rename a `tdd_sabotage_records` en 42.5, NO drop); `saga_compensation_log` se DROPEA (entra a este lote). Pendientes: `sessions` (¿drop o épica aparte?), `runtime_configs` (¿eliminar feature o reimplementar?)
- [ ] `grep -rn "lifecycletrack" --include=*.go` → confirmar 0 importers fuera del pkg
- [ ] Mapear las FK entrantes a `sessions`: `captured_prompts.session_id`, `verifications.session_id` (confirmado)
- [ ] Confirmar 0 FK entrantes a `saga_compensation_log` (drop limpio sin CASCADE)

## Código a remover — `entity_state_transitions` (SAFE)

- [ ] Borrar el pkg `internal/service/lifecycletrack/` completo (código muerto, sin importers). Es el único lugar con SQL real (`service.go` l.198-249).
- [ ] `internal/service/orchestrator/phases/sdd_archive.go` (l.31): actualizar el TEXTO de prompt que menciona "registra la transición en entity_state_transitions" (es string, no rompe build).
- [ ] `internal/seeds/agent_templates_catalog.go` (l.589): texto de prompt menciona la tabla (string, opcional).

## Código a remover — `system_state`

- [ ] `internal/scheduler/cron/system/orphan_runs_audit.go` (l.135-165): reescribir `getLastAck`/`setLastAck` para NO usar `system_state` (eliminar el cursor persistido, o moverlo a `flow_config`).
- [ ] `cmd/domain/main.go` (l.748-750): si se elimina el feature, quitar wiring del `OrphanAuditor` (`OrphanAuditEnabled`).
- [ ] `internal/api/handler/dbschema.go` (l.230): quitar referencia a `system_state` en la clasificación/ocultamiento de tablas.

## Código a refactorizar — `saga_compensation_log` (drop)

- [ ] `internal/service/flow/saga.go`: quitar/stubear los literales SQL que tocan `saga_compensation_log` (INSERT/SELECT del log de compensación). Si la compensación saga ya no persiste, eliminar el store; si se mantiene en memoria, dejar passthrough.
- [ ] `internal/scheduler/cron/system/heartbeat_watcher.go`: quitar la referencia a `saga_compensation_log`.
- [ ] `tests/e2e/heartbeat_watcher_test.go` y `tests/e2e/schema_audit_test.go`: actualizar/eliminar las aserciones sobre `saga_compensation_log`.

> **`sabotage_records` NO se toca en esta HU** (decisión: se preserva). Su código (`task/service.go` CreateSabotage/ListSabotages, `sdd_judge.go`, `registry.go`) sigue vivo; los literales SQL se actualizan a `tdd_sabotage_records` en la HU 42.5, no aquí.

## Código a remover — `model_registry`

- [ ] `internal/service/agent/pg_repository.go` (l.146-155): eliminar/stubear `ModelExists` (`SELECT EXISTS FROM model_registry`) → `true`.
- [ ] `internal/service/agent/service.go`: quitar `validateModel` (l.150) y `ErrModelUnknown` (l.40) — sin gating por tabla.
- [ ] `internal/service/agent/repository.go` (l.12,25): quitar `ModelExists` del interface.
- [ ] `internal/llm/registry/registry.go`: borrar pkg o reescribir `loadLocked` (l.118-125) para no leer `model_registry`; mover pricing (input/output_per_million) a CONSTANTES en código.
- [ ] `internal/runner/agent/runner.go`: adaptar el uso de `llm/registry` para pricing.
- [ ] `internal/runner/flow/dryrun.go` (l.92): quitar referencia a `model_registry`.
- [ ] `internal/seeds/model_registry_seeder.go`: borrar archivo (seeder).
- [ ] `cmd/domain/main.go` (l.486) y `cmd/domain/install_cli.go` (l.1136): quitar `Register(&seeds.ModelRegistrySeeder{})`.
- [ ] `internal/cache/distributed/cache.go` (l.5): comentario "model_registry pricing" (cosmético).
- [ ] `internal/dbconvlint/lint.go` (l.238): quitar `model_registry` de `commonNonPluralAllowed` (sino el linter referencia tabla inexistente).

## Código a remover — `runtime_configs` (ALTO RIESGO: rompe boot/SIGHUP/cron/rutas)

- [ ] `internal/runtimeconfig/registry.go`: borrar pkg (Update/Refresh/Query a `runtime_configs`) o reimplementar sobre otra fuente. **Esta HU: eliminar.**
- [ ] `cmd/domain/main.go`: quitar Registry runtimeconfig (l.460-461), su `Refresh` en cron (l.876), el SIGHUP handler (l.1071-1079) y el import (l.51).
- [ ] `internal/api/handler/runtimeconfig.go`: borrar handlers `getRuntimeConfig`/`updateRuntimeConfig`.
- [ ] `internal/api/handler/api.go`: quitar campo `RuntimeConfigRegistry` (l.124), import (l.64) y rutas `GET/POST /api/v1/admin/runtime-configs/{key}` (l.431-432).

## Código a remover — `dead_letter_queue` (ALTO RIESGO: inyectado en el flow runner)

- [ ] `internal/service/flow/dlq.go`: borrar `DLQStore` (INSERT/SELECT/resolve l.43-88).
- [ ] `internal/runner/flow/runner.go`: quitar campo `DLQ` y los push al DLQ en el path de fallo (l.88, l.292-301).
- [ ] `cmd/domain/main.go` (l.684): quitar `DLQ: &flow.DLQStore{Pool: pools.App}` de la construcción del Runner.
- [ ] `internal/api/handler/dlq.go`: borrar handlers `listDLQ`/`resolveDLQ`.
- [ ] `internal/api/handler/api.go` (l.370-371): quitar rutas `GET /api/v1/dlq` y `DELETE /api/v1/dlq/{id}` + wiring del store.
- [ ] `internal/api/handler/dbschema.go`: quitar referencia a `dead_letter_queue`/DLQ en clasificación de tablas.

## Código a remover — `idempotency_keys` (ALTO RIESGO: middleware en TODA request mutante)

- [ ] `internal/api/middleware/idempotency.go`: borrar el middleware `Idempotency` (SELECT/INSERT `idempotency_keys` l.124-207) o stubear a passthrough.
- [ ] `cmd/domain/main.go`: quitar `idempMW := &middleware.Idempotency{Pool: pools.App}` (l.949) y el `idempMW.Wrap(...)` de la cadena (l.976).
- [ ] `internal/api/middleware/cors.go` (l.48): quitar header `Idempotency-Key` del allowlist CORS si ya no se usa.

## Código a remover — `sessions` (MAYOR RIESGO: feature completa + paquetes KEEP)

- [ ] `internal/service/session/{repository.go,pg_repository.go,service.go}`: borrar pkg completo (CRUD de sessions).
- [ ] `internal/api/handler/session.go`: borrar handler y sus rutas en `api.go` (l.318-322).
- [ ] `cmd/domain/main.go` y `cmd/domain-mcp/main.go`: quitar construcción/inyección de `session.Service` y dependientes.
- [ ] `internal/mcp/server/{server.go,memory_tools.go,health_tools.go,wireup.go}`: quitar MCP tools que crean/leen sessions (`server.go` l.192-407, `memory_tools` l.260-322, `health_tools` l.91).
- [ ] **REFACTOR (no borrar — son KEEP)** `internal/service/timeline/service.go`: reescribir `querySessions`/`ActiveSession` (l.48-223) sin dependencia de `sessions`.
- [ ] **REFACTOR** `internal/service/search/service.go`: quitar search sobre `sessions` (l.113-248).
- [ ] **REFACTOR** `internal/service/lifecycle/{erasure.go,service.go}`: quitar borrado GDPR de `sessions` (erasure l.85, service l.41-225).
- [ ] **REFACTOR** `internal/context/stitcher/stitcher.go`: quitar lectura de `sessions` para stitching (l.69-100).
- [ ] Quitar la columna `session_id` de los structs/queries de los repos de `captured_prompts` y `verifications`.
- [ ] `services/domain-admin/template/src/app/views/admin-maintainers/maintainer-registry.ts` (l.84-86): quitar entry `sessions` → `/api/v1/sessions`.

## Migration 000149

- [ ] Escribir `000149_drop_legacy_infra.up.sql` con header completo (migration/author/issue/description/breaking/estimated_duration).
- [ ] `up`: `ALTER TABLE IF EXISTS captured_prompts DROP CONSTRAINT IF EXISTS captured_prompts_session_id_fkey;` + `DROP COLUMN IF EXISTS session_id;` + idem `verifications` (constraint `verifications_session_id_fkey`); luego `DROP TABLE IF EXISTS <tabla>` por las 8 (CASCADE salvo `saga_compensation_log`, que es limpia); `DROP FUNCTION IF EXISTS entity_state_transitions_immutable() CASCADE;`. Todo en `BEGIN/COMMIT`. Las 8: sessions, model_registry, entity_state_transitions, system_state, saga_compensation_log, runtime_configs, dead_letter_queue, idempotency_keys.
- [ ] `down`: recrear las 8 tablas con shape **post-Fase-C** (sin `organization_id`, con `status`), índices, triggers `set_updated_at_*`, función+trigger+view de `entity_state_transitions`, esqueleto de `saga_compensation_log` (run_id → flow_runs + sus 3 índices), grants; y re-`ADD COLUMN session_id` (FK nullable ON DELETE SET NULL) a `captured_prompts` y `verifications`. NO recrear `sabotage_records` (no es parte del drop).
- [ ] Verificar que pasa squawk (DROP en tablas vacías, sin lock largo).
- [ ] Verificar roundtrip up→down→up en DB fresca.

## Sabotaje (anti-falsos positivos) — OBLIGATORIO

Objetivo: probar que los tests detectan REALMENTE la dependencia muerta, no que pasan por casualidad.

- [ ] **Sabotaje 1 (DB):** después de aplicar 000149 up, re-introducir a mano un `SELECT 1 FROM runtime_configs LIMIT 1` en `internal/runtimeconfig/registry.go` y reconstruir el wiring en main.go. Correr el binario.
  - **Esperado:** el boot FALLA con `relation "runtime_configs" does not exist` (o el build falla si el pkg fue borrado). Confirma que la dependencia estaba viva y que el drop la rompe de verdad.
  - Restaurar el fix (pkg removido) → boot OK.
- [ ] **Sabotaje 2 (FK entrante):** comentar el `ALTER TABLE ... DROP COLUMN session_id` de `captured_prompts`/`verifications` en la migration y correr `up`.
  - **Esperado:** `DROP TABLE sessions CASCADE` o bien borra silenciosamente la columna (CASCADE) — para evitar el CASCADE silencioso, el test verifica EXPLÍCITAMENTE que `session_id` ya NO existe en `captured_prompts`/`verifications` tras el up. Si el DROP COLUMN se omite, el test de "columna no existe" debe seguir pasando SOLO si CASCADE la quitó; si no, FALLA. Documentar el comportamiento observado.
  - Restaurar el `DROP COLUMN` explícito (no depender del CASCADE silencioso).
- [ ] **Sabotaje 3 (model_registry):** stubear `ModelExists` para que devuelva `false` en vez de `true`.
  - **Esperado:** el test de "crear agente con modelo válido" FALLA con `ErrModelUnknown` (o equivalente). Confirma que el stub correcto (`true`) es el que mantiene viva la creación de agentes.
  - Restaurar `true`.
- [ ] Después de cada sabotaje: restaurar el fix → el test vuelve a verde.

## Cierre

- [ ] `go vet ./...` sin warnings
- [ ] `go build ./...` OK (sin referencias a las 8 tablas)
- [ ] `go test ./...` verde (incluye roundtrip de migration + smoke de boot)
- [ ] `domain-lint` / squawk sobre 000149 sin findings
- [ ] Verificación manual: `migrate up` en DB fresca + boot del binario + crear un agente + disparar un flow run de fallo (ya sin DLQ)
- [ ] Confirmar que `timeline`/`search`/`lifecycle`/`stitcher` (KEEP) siguen con sus tests verdes tras el refactor
- [ ] Commit en rama `services` (Conventional Commits, español, SIN Co-Authored-By):
      `chore(req-42.3): drop 8 tablas legacy/infra + remoción de código acoplado`
- [ ] NO git push (repo local-only)
