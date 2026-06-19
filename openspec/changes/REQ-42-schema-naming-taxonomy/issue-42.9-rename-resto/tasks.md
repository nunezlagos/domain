# Tasks: issue-42.9-rename-resto

## Verificación previa (bloqueante)

- [ ] Confirmar que 000155 está libre y no colisiona con 000151 (42.5), 000152 (42.6), 000153 (42.7)
- [ ] Confirmar que 42.5/42.7 cubren enrollment_tokens/verifications/verification_results (de-scope válido)
- [ ] Confirmar contra la introspección que las 9 tablas NO tienen sequence (PK UUID)
- [ ] Confirmar que NINGUNA de las 9 tiene RLS policy activa (`relrowsecurity=f`; FORCE sin ENABLE inerte)
- [ ] Confirmar el orden de ejecución respecto al DROP de `sessions` (coordinación de `session_id` en captured_prompts)
- [ ] Si otra HU ya absorbió estos renames → marcar HU como no-op de verificación y NO emitir migration

## Migration 000155 (par up + down)

- [ ] Red: test de migración verifica `to_regclass` para las 9 tablas (nuevo existe, viejo no) tras `up`
- [ ] Escribir `000155_rename_resto_taxonomy.up.sql` con header completo (migration/author/issue/description/breaking/estimated_duration) y `BEGIN/COMMIT`
- [ ] Confirmar que la migration NO contiene ALTER sobre enrollment_tokens, verifications ni verification_results (de-scope)
- [ ] `prompt_captured`: SOLO `ALTER INDEX` para el pkey (NO `RENAME CONSTRAINT` sobre pkey → duplicado)
- [ ] `imported_workflow_files`: SOLO `ALTER INDEX` para `*_unique` (constraint con índice homónimo)
- [ ] Pares webhook (subs+deliveries) y runner (runners+tasks) en la MISMA tx
- [ ] Escribir `000155_rename_resto_taxonomy.down.sql` como reverso EXACTO de las 9 tablas
- [ ] Usar `IF EXISTS` donde aplique sin romper atomicidad (renames no admiten IF EXISTS directo en ALTER INDEX antiguo; el reverso es exacto por diseño)
- [ ] Verificar que ambos archivos pasan `squawk` (renames metadata-only, sin reescritura)
- [ ] Green: aplicar `up` en DB de prueba, verificar schema
- [ ] Red→Green: aplicar `down`, verificar retorno exacto al estado original

## Touchpoints de código (rename del identificador SQL en Go)

> NOTA: los touchpoints de `enrollment_tokens` (HU 42.7), `verifications` y `verification_results` (HU 42.5) NO están aquí — pertenecen a esas HU.

### captured_prompts → prompt_captured
- [ ] `internal/service/capturedprompt/pg_repository.go` — repositorio principal
- [ ] `internal/mcp/server/captured_prompt_tools.go` — herramientas MCP
- [ ] `internal/api/handler/org_overview.go` — COUNT/SELECT de overview
- [ ] `internal/api/handler/rest_new.go` — referencia en handler REST
- [ ] `internal/mcp/server/health_tools.go` (~línea 107) — `SELECT COUNT(*) FROM captured_prompts WHERE user_id=$1`
- [ ] `cmd/domain/seed_demo.go` — seeder demo

### clients → project_clients
- [ ] `internal/service/client/pg_repository.go` — CRUD completo
- [ ] `internal/service/project/pg_repository.go` — JOIN/FROM clients
- [ ] `internal/service/project/service.go` — resolución de cliente del proyecto
- [ ] `internal/mcp/server/health_tools.go` (~línea 88) — `SELECT COUNT(*) FROM clients WHERE deleted_at IS NULL`
- [ ] `cmd/domain/seed_demo.go` — seeder demo

### imported_workflow_files → project_imported_workflow_files
- [ ] `internal/service/workflowimport/service.go` — CRUD (incl. unique project_id+rel_path)
- [ ] `cmd/domain/init_cli.go` — referencia en init CLI
- [ ] `cmd/domain/main.go` — referencia

### observations → knowledge_observations (ALTO FAN-OUT, 12 archivos)
- [ ] `internal/service/observation/pg_repository.go` — repositorio principal (dedup_hash, embedding ivfflat, tags GIN)
- [ ] `internal/mcp/server/memory_tools.go` — herramientas MCP de memoria/RAG
- [ ] `internal/mcp/server/session_bootstrap_tools.go` — bootstrap de sesión
- [ ] `internal/service/search/service.go` — búsqueda híbrida (tsv + embedding)
- [ ] `internal/service/timeline/service.go` — timeline
- [ ] `internal/service/usage/service.go` — conteo/uso
- [ ] `internal/service/projectmerge/service.go` — reasignación project_id
- [ ] `internal/service/lifecycle/service.go` — soft delete / retención
- [ ] `internal/service/lifecycle/erasure.go` — borrado/anonimización GDPR
- [ ] `internal/api/handler/export.go` — export de datos
- [ ] `internal/mcp/server/health_tools.go` (~línea 90) — `SELECT COUNT(*) FROM observations WHERE deleted_at IS NULL`
- [ ] `cmd/domain/seed_demo.go` — seeder demo

### outbound_webhook_subscriptions → webhook_outbound_subscriptions
- [ ] `internal/service/outboundwebhook/service.go` — CRUD subscriptions (events GIN, url)
- [ ] `internal/service/outboundwebhook/dispatcher.go` — SELECT subs activas por evento
- [ ] `internal/anonymizer/types.go` — clave de tabla en mapa de anonimización/PII

### outbound_webhook_deliveries → webhook_outbound_deliveries
- [ ] `internal/service/outboundwebhook/dispatcher.go` — INSERT/UPDATE deliveries, retry
- [ ] `internal/api/handler/outboundwebhook.go` — handler REST lista deliveries
- [ ] `internal/api/backpressure/backpressure.go` — COUNT deliveries pending

### selfhosted_runners → runner_selfhosted  /  selfhosted_tasks → runner_selfhosted_tasks
- [ ] `internal/runner/selfhosted/selfhosted.go` — registro, heartbeat, claim (cubre AMBAS tablas en un solo archivo)

### activity_log → audit_activity_log
- [ ] `internal/activity/activity.go` — INSERT/SELECT (actor, entity, project, visibility)
- [ ] Confirmar que la mención en `internal/seeds/platform_policies_seeder.go` es TEXTO documental (falso positivo), NO identificador SQL → NO tocar

## Verificación de identificadores (anti-omisión)

- [ ] `grep -rnw observations internal/ cmd/` y cruzar contra la lista de 12 archivos (descartar comentarios/strings no-SQL)
- [ ] `grep -rnw clients internal/ cmd/` (cuidado: no confundir con `oauth clients` u otros usos)
- [ ] `grep -rn captured_prompts internal/ cmd/`
- [ ] `grep -rn outbound_webhook internal/ cmd/`
- [ ] `grep -rn selfhosted_ internal/ cmd/`
- [ ] `grep -rn imported_workflow_files internal/ cmd/`
- [ ] `grep -rn activity_log internal/ cmd/`

## Admin /database

- [ ] Verificar que `database-explorer.component.ts` lista las tablas por nombre real (no hay lista hardcodeada con nombres viejos)
- [ ] Si hay agrupación por prefijo en el explorer, confirmar que las nuevas tablas caen en su grupo (`prompt_`, `project_`, `tdd_`, `knowledge_`, `webhook_`, `runner_`, `audit_`)

## Sabotaje (anti-falsos positivos)

OBLIGATORIO. Dos sabotajes reales:

- [ ] **Sabotaje A (migration — duplicado de pkey):** agregar al `up` un `ALTER TABLE prompt_captured RENAME CONSTRAINT captured_prompts_pkey TO prompt_captured_pkey` (además del `ALTER INDEX captured_prompts_pkey` ya presente). Aplicar la migration. **Esperado:** FALLA con error de constraint inexistente / nombre ya en uso (el `ALTER INDEX` ya renombró el objeto compartido pkey). Restaurar (quitar el `RENAME CONSTRAINT` redundante) → la migration pasa.
- [ ] **Sabotaje B (código — query sin renombrar):** dejar a propósito `internal/service/observation/pg_repository.go` con UNA query que sigue diciendo `FROM observations` después de aplicar la migration (que ya renombró a `knowledge_observations`). Correr el test de integración del repositorio. **Esperado:** FALLA con `relation "observations" does not exist`. Confirma que el test detecta queries no migradas. Restaurar (renombrar a `knowledge_observations`) → test verde.
- [ ] Después de cada sabotaje: restaurar el fix y confirmar que la suite vuelve a verde.

## Cierre

- [ ] `go vet ./...` sin warnings
- [ ] `go build ./...` OK
- [ ] `go test ./...` verde (incluye tests de migración up/down e integración de repositorios tocados)
- [ ] `squawk` verde sobre `000155_*.up.sql` y `000155_*.down.sql`
- [ ] Verificar manualmente en DB de prueba: `up` → 9 tablas renombradas; `down` → 9 tablas restauradas
- [ ] Confirmar que las FK entrantes (a `project_clients`) y los pares (webhook, runner) siguen válidos post-`up`
- [ ] Commit en rama `services` (Conventional Commits, español, SIN Co-Authored-By):
      `refactor(schema): rename resto de tablas a prefijo de grupo (REQ-42.9, migration 000155)`
- [ ] NO git push (repo local-only)
