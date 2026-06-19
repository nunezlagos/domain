# Design: issue-42.4-tabla-agent-run-prompts

## Decisión arquitectónica

**Tabla satélite de `agent_runs`, una fila por `(agent_run_id, iteration)`, escrita best-effort por el orquestador.** `agent_run_prompts` captura la "verdad de runtime" del lado ENTRADA del LLM: el `system_prompt` resuelto del agente más el array de `messages` (user/assistant/tool) que realmente se serializa y se envía en `provider.Complete()`. Sigue el mismo patrón estructural que `agent_run_logs` (FK a `agent_runs(id)` ON DELETE CASCADE, escritura inline en el loop, errores silenciados).

**Por qué una tabla nueva y NO reusar `agent_run_logs`:** los dos ejes son opuestos y conviene NO solaparlos.

- `agent_run_logs` = lo que SALIÓ del LLM (event_type `llm_call` con `content`/`tool_calls`/`finish_reason`, más `tool_call`/`tool_result`/`error`). Es el registro de respuestas y efectos.
- `agent_run_prompts` = lo que ENTRÓ al LLM (system + messages + tools defs) ANTES de la llamada. Es el registro del request.

Mezclarlos forzaría a inferir el request reconstruyendo el historial de logs; tenerlos separados permite reproducir un run exactamente con lo que se le mandó al modelo.

**Por qué NO reusar `captured_prompts`:** ver la tabla comparativa en `issue.md`. `captured_prompts` es input crudo del HUMANO (raw_text del IDE) para analizar calidad de redacción; no conoce el agente ni el run. `agent_run_prompts` es output de la PLATAFORMA hacia el MODELO. Son extremos del mismo eje (humano → plataforma → modelo).

**Single-org:** la migración 000132 deshabilitó RLS y la 000142 dropeó `organization_id` en TODAS las tablas. Por lo tanto `agent_run_prompts` se crea SIN `organization_id` y SIN RLS, a diferencia de `captured_prompts` (000104), que sí los traía (hoy inertes). El aislamiento de tenant ya no vive en el schema.

## DDL (migration 000150)

```sql
CREATE TABLE IF NOT EXISTS agent_run_prompts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  agent_run_id UUID NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
  iteration INT NOT NULL DEFAULT 0,            -- 0 = pre-flight, 1..N = cada llamada al LLM
  model VARCHAR(100),
  system_prompt TEXT NOT NULL DEFAULT '',
  messages JSONB NOT NULL DEFAULT '[]',        -- user/assistant/tool tal cual van a opts.Messages
  tool_slugs TEXT[] NOT NULL DEFAULT '{}',     -- slugs de las tools expuestas en la llamada
  char_count INT NOT NULL DEFAULT 0,           -- proxy de tamano (len system + len de cada message)
  estimated_tokens INT NOT NULL DEFAULT 0,     -- tokens.EstimateMessages(system_prompt, messages)
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT agent_run_prompts_iteration_check CHECK (iteration >= 0),
  CONSTRAINT agent_run_prompts_run_iter_key UNIQUE (agent_run_id, iteration)
);

CREATE TRIGGER set_updated_at_agent_run_prompts
  BEFORE UPDATE ON agent_run_prompts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX IF NOT EXISTS agent_run_prompts_run_idx     ON agent_run_prompts (agent_run_id, iteration);
CREATE INDEX IF NOT EXISTS agent_run_prompts_created_idx ON agent_run_prompts (created_at DESC);
```

Down: `DROP TABLE IF EXISTS agent_run_prompts CASCADE;` (mismo patrón que `000015_create_agent_runs.down.sql`; el CASCADE arrastra trigger e índices).

**Notas squawk:**
- `CREATE TABLE IF NOT EXISTS` y `CREATE INDEX IF NOT EXISTS` → idempotente, sin warning de "table already exists".
- Tabla nueva sin filas → no hay backfill ni lock peligroso sobre datos existentes. La DB hoy está casi vacía (riesgo de datos nulo).
- El `UNIQUE` se declara inline en el `CREATE TABLE`, no como `ALTER TABLE ADD CONSTRAINT` posterior, por lo que no toma lock sobre una tabla con datos.
- Todo dentro de `BEGIN/COMMIT` (precedente: 000146).

## Punto de inserción en el orquestador

Archivo: `internal/runner/agent/runner.go`, función `Run()`, dentro del `LOOP:` (líneas 198-267).

Estado en scope verificado:
- `runID uuid.UUID` creado en líneas 158-163 (INSERT en `agent_runs`).
- `iterations int` incrementado en línea 200 (`iterations++`).
- `opts llm.CompletionOptions` armado en líneas 201-206 con `Model`, `SystemPrompt`, `Messages`, `Tools`.
- `tools []llm.ToolDef` y `skillBySlug` resueltos en línea 145 (`loadSkillTools`).
- `tokens.EstimateMessages` importado y usado ya en el pre-flight (línea 128).

Snapshot a insertar JUSTO ANTES de `resp, err := provider.Complete(ctx, opts)` (línea 218):

```go
// snapshot del prompt efectivo que cae al agente en esta iteracion (REQ-42.4).
// Best-effort: un error de persistencia NO debe abortar el run (patron appendLog).
r.appendPromptSnapshot(ctx, runID, iterations, opts, tools)

callStart := time.Now()
resp, err := provider.Complete(ctx, opts)
```

Helper nuevo, análogo a `appendLog` (cerca de la línea 428):

```go
// appendPromptSnapshot persiste el prompt efectivo enviado al LLM en una
// iteracion (issue-42.4: agent_run_prompts). Best-effort: errores silenciados.
func (r *Runner) appendPromptSnapshot(ctx context.Context, runID uuid.UUID,
    iteration int, opts llm.CompletionOptions, tools []llm.ToolDef) {
    if r.Pool == nil {
        return
    }
    msgsJSON, _ := json.Marshal(opts.Messages)
    slugs := make([]string, 0, len(tools))
    for _, t := range tools {
        slugs = append(slugs, t.Name)
    }
    est := tokens.EstimateMessages(opts.SystemPrompt, opts.Messages)
    cc := len(opts.SystemPrompt)
    for _, m := range opts.Messages {
        cc += len(m.Content)
    }
    _, _ = r.Pool.Exec(ctx,
        `INSERT INTO agent_run_prompts
           (agent_run_id, iteration, model, system_prompt, messages, tool_slugs, char_count, estimated_tokens)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
         ON CONFLICT (agent_run_id, iteration) DO UPDATE
           SET messages = EXCLUDED.messages,
               char_count = EXCLUDED.char_count,
               estimated_tokens = EXCLUDED.estimated_tokens,
               updated_at = NOW()`,
        runID, iteration, opts.Model, opts.SystemPrompt, msgsJSON,
        slugs, cc, est)
}
```

> Verificar la firma real de `tokens.EstimateMessages` antes de codear; en el pre-flight (línea 128) se invoca como `tokens.EstimateMessages(agent.SystemPrompt, []llm.Message{...})`, por lo que la firma esperada es `(systemPrompt string, messages []llm.Message) int`.

## Convención de `iteration`

- `iteration = 0` → reservado para pre-flight (el prompt que IBA a caer si el run muere antes del loop: quota/provider_missing/load_skills). El código del loop arranca en `iteration = 1`. La UI NO debe asumir que la numeración empieza en 1.
- `iteration = 1..N` → una fila por cada llamada efectiva a `provider.Complete()`.

Opcional (cobertura 100%): capturar también en `failedRun()` (~línea 403-424) con `iteration = 0` para no perder la evidencia del prompt cuando el run muere antes del loop. Se documenta como tarea opcional en `tasks.md`.

## Riesgos y mitigaciones

| Riesgo | Mitigación |
|---|---|
| **Volumen**: 1 fila por iteración (no por run); `messages` crece acumulando assistant+tool en cada iter → JSONB grande con muchas filas | Hoy la DB está casi vacía (riesgo nulo). A escala: truncar/comprimir `messages` o guardar solo el delta. Documentado, no implementado ahora. |
| **Duplicación de storage**: `messages` solapa lo que `agent_run_logs` ya guarda parcialmente | Frontera clara: `agent_run_logs` = lo que SALIÓ del LLM; `agent_run_prompts` = lo que ENTRÓ. No se reescribe `agent_run_logs`. |
| **Hot path**: el INSERT corre en el loop síncrono antes de cada llamada al LLM | Es un `Exec` extra por iteración; despreciable frente a la latencia del LLM. Best-effort (`_, _ = ...`): si falla NO aborta el run, igual que `appendLog`. |
| **PII/secretos**: `system_prompt` y `messages` pueden contener credenciales inyectadas o contenido de proyecto | Single-org mitiga aislamiento. El endpoint admin que exponga la tabla debe respetar RBAC; evaluar masking en la UI `/database`. |
| **UNIQUE (run, iteration) + retry**: un retry de la misma iteración sobrescribe el snapshot previo vía `ON CONFLICT DO UPDATE` y se pierde el intento fallido | Aceptable para el caso actual. Si se necesita historial por intento: quitar el UNIQUE y agregar `attempt_no`. Documentado. |
| **FK ON DELETE CASCADE**: al purgar un `agent_run` se pierden sus prompts | Correcto para retención atada al run. Si se necesita auditoría a largo plazo independiente del run, archivar antes del delete. |
| **`iteration = 0` mal interpretado por la UI** | Documentar la convención: 0 = pre-flight, 1..N = llamadas reales. |

## TDD plan

1. **Red**: test de migración — aplicar `000150.up` sobre una DB de test y assert: tabla existe, columnas/tipos correctos, NO existe `organization_id`, NO hay RLS, FK a `agent_runs` con CASCADE, UNIQUE `(agent_run_id, iteration)`, trigger `set_updated_at_agent_run_prompts` presente.
2. **Green**: escribir la migración up/down.
3. **Red**: test de `appendPromptSnapshot` — con un run insertado, llamar el helper con un `opts` de prueba y assert que aparece 1 fila con `system_prompt`, `messages` (JSONB roundtrip), `tool_slugs`, `char_count` y `estimated_tokens` correctos.
4. **Green**: implementar el helper + la llamada en el loop.
5. **Red**: test de idempotencia — llamar el helper dos veces con el mismo `(runID, iteration)` y assert que hay 1 sola fila, con los valores actualizados y `updated_at > created_at`.
6. **Green/Refactor**: confirmar `ON CONFLICT DO UPDATE`.
7. **Sabotaje**: ver `tasks.md` (forzar fallo del INSERT y confirmar que el run NO aborta).
8. **Down**: aplicar `000150.down` y assert que la tabla ya no existe.
