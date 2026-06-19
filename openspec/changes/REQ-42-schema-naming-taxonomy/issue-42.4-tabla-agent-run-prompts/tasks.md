# Tasks: issue-42.4-tabla-agent-run-prompts

## Verificación previa (bloqueante)

- [ ] Confirmar que `000146` es la última migration y que `000150_create_agent_run_prompts.{up,down}.sql` no existe aún
- [ ] Confirmar que `set_updated_at()` existe globalmente (mig 000001-000003)
- [ ] Confirmar que `agent_runs(id)` es la PK destino de la FK (patrón `agent_run_logs`)
- [ ] Confirmar que las tablas runtime ya NO tienen `organization_id` ni RLS (mig 000142 / 000132)
- [ ] Confirmar firma de `tokens.EstimateMessages(systemPrompt string, messages []llm.Message) int` (uso real en `runner.go:128`)
- [ ] Confirmar scope dentro del loop de `Run()`: `runID` (158-163), `iterations` (200), `opts` (201-206), `tools` (145)

## Migration — `000150_create_agent_run_prompts.{up,down}.sql`

- [ ] `up.sql`: header completo (migration/author/issue/description/breaking/estimated_duration)
- [ ] `up.sql`: `BEGIN/COMMIT`, `CREATE TABLE IF NOT EXISTS agent_run_prompts` con FK ON DELETE CASCADE
- [ ] `up.sql`: `CHECK (iteration >= 0)` + `UNIQUE (agent_run_id, iteration)` inline (sin ALTER posterior)
- [ ] `up.sql`: trigger `set_updated_at_agent_run_prompts` con `set_updated_at()`
- [ ] `up.sql`: índices `agent_run_prompts_run_idx (agent_run_id, iteration)` y `agent_run_prompts_created_idx (created_at DESC)` con `IF NOT EXISTS`
- [ ] `down.sql`: `DROP TABLE IF EXISTS agent_run_prompts CASCADE;` (patrón 000015)
- [ ] Verificar que NO se incluye `organization_id` ni `ENABLE ROW LEVEL SECURITY`
- [ ] `squawk` pasa sobre `up.sql` y `down.sql` sin findings de lock/backfill
- [ ] Aplicar up → down → up en DB de test (roundtrip limpio)

## Backend — `internal/runner/agent/runner.go`

- [ ] Implementar helper `appendPromptSnapshot(ctx, runID, iteration, opts, tools)` análogo a `appendLog` (cerca línea 428)
- [ ] Helper: `if r.Pool == nil { return }` (guard idéntico a `appendLog`)
- [ ] Helper: `json.Marshal(opts.Messages)` → `messages`; recorrer `tools` para `tool_slugs`; `tokens.EstimateMessages` → `estimated_tokens`; sumar `len(SystemPrompt)+len(m.Content)` → `char_count`
- [ ] Helper: `INSERT ... ON CONFLICT (agent_run_id, iteration) DO UPDATE` con `_, _ = r.Pool.Exec(...)` (errores silenciados)
- [ ] Llamar `r.appendPromptSnapshot(ctx, runID, iterations, opts, tools)` JUSTO ANTES de `provider.Complete(ctx, opts)` (línea 218)
- [ ] (Opcional, cobertura 100%) capturar también en `failedRun()` con `iteration = 0` para el prompt que iba a caer antes del loop

## Tests

- [ ] Test migración: tabla existe, columnas/tipos OK, FK CASCADE, UNIQUE `(agent_run_id, iteration)`, CHECK `iteration >= 0`, trigger presente, SIN `organization_id`, SIN RLS
- [ ] Test migración down: tras `down.sql` la tabla no existe
- [ ] Test helper `appendPromptSnapshot`: inserta 1 fila con `system_prompt`/`messages` (roundtrip JSONB)/`tool_slugs`/`char_count`/`estimated_tokens` correctos
- [ ] Test idempotencia: dos llamadas con mismo `(runID, iteration)` → 1 fila, valores actualizados, `updated_at > created_at`
- [ ] Test cardinalidad: un run con 3 iteraciones → 3 filas con `iteration` 1, 2, 3
- [ ] Test cascade: `DELETE FROM agent_runs WHERE id = R` → 0 filas en `agent_run_prompts` para R
- [ ] Test integración loop: correr `Run()` con provider fake (1 tool call + 1 final) y assert que `messages` de la iteración 2 incluye assistant+tool

## Sabotaje (anti-falsos positivos)

**Objetivo:** demostrar que el snapshot del prompt es best-effort y que un fallo de persistencia NUNCA aborta el run, y que el test que lo cubre NO es un falso positivo.

- [ ] **Sabotaje del fix (el run debe sobrevivir):** inyectar un `r.Pool` mock cuyo `Exec` del INSERT de `agent_run_prompts` devuelva un error (p.ej. `errors.New("boom")`). Correr `Run()` con un provider fake que responde sin tool calls. **Esperado:** `Run()` completa con `Status = completed`, devuelve `finalText`, y el error del snapshot se traga (no aparece en `RunResult.Error`). Si el run abortara, el helper NO sería best-effort → BUG.
- [ ] **Sabotaje del test de cardinalidad (debe FALLAR):** romper a propósito el helper cambiando `runID, iterations` por `runID, 0` (todas las iteraciones escriben `iteration = 0`). **Esperado:** el `ON CONFLICT (agent_run_id, iteration)` colapsa las 3 filas en 1 → el test de cardinalidad (3 filas, iteration 1/2/3) FALLA. Confirma que el test realmente verifica la numeración por iteración y no pasa por casualidad.
- [ ] **Sabotaje del test de cascade (debe FALLAR):** romper a propósito la FK quitando `ON DELETE CASCADE` en la migración. **Esperado:** el `DELETE FROM agent_runs` falla por violación de FK (o deja filas huérfanas según el modo) → el test de cascade FALLA. Confirma que el CASCADE es real.
- [ ] **Restaurar:** revertir las tres mutaciones de sabotaje → los tres tests vuelven a verde.

## Cierre

- [ ] `go vet ./...` sin warnings
- [ ] `go build ./...` OK
- [ ] `go test ./internal/runner/agent/... ./internal/migrate/...` verde
- [ ] `squawk` verde sobre las dos migrations
- [ ] Verificación manual: aplicar la migración en local, correr un agent_run real y `SELECT iteration, model, char_count, estimated_tokens, tool_slugs FROM agent_run_prompts WHERE agent_run_id = '<run>' ORDER BY iteration`
- [ ] Commit en rama `services` (Conventional Commits, español, SIN Co-Authored-By): `feat(agent): tabla agent_run_prompts + snapshot del prompt por iteracion`
- [ ] NO `git push` (repo local-only)
