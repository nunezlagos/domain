# Design: issue-42.3-drop-legacy-infra

## Decisión arquitectónica

**Drop atómico en una sola transacción + remoción de código en el mismo deploy.** Las 8 tablas se dropean en una única migration (`000149`) dentro de `BEGIN/COMMIT`, con `DROP TABLE IF EXISTS ... CASCADE` para idempotencia. PERO 6 de las 8 tienen código vivo: la migration NO se aplica en aislamiento — el deploy completo incluye la remoción/refactor del código Go que las usa. Si el código no se toca:

- El binario **no compila** (`runtimeconfig`, `idempotency`, `flow.DLQStore`, `session.Service`, `model_registry` queries).
- O compila pero **no bootea / falla en runtime** (`runtimeconfig.Refresh()` al arranque, `idempMW.Wrap()` en la cadena HTTP, `OrphanAuditor` leyendo `system_state`).

**Orden de deploy obligatorio (code-first, luego migration):**

```
1. Remover/refactorizar el código Go (todas las secciones de tasks.md)
2. go build ./... + go vet ./... + go test ./...   ← verde ANTES de tocar la DB
3. Aplicar migration 000149 up (drop de las 8 tablas)
4. Re-deploy del binario ya sin referencias a las tablas
```

> Razón del code-first: si dropeo la tabla primero, el binario en producción (todavía con el código viejo) crashea al hacer `runtimeconfig.Refresh()` o al recibir una request mutante (idempotency middleware hace `INSERT INTO idempotency_keys`). Code-first garantiza que en ningún instante el binario corriendo referencia una tabla inexistente.

**Precedente atómico:** migration `000146` (rename `org_flow_config`→`flow_config`) y `000143` (drop `organizations` CASCADE) — mismo patrón `BEGIN/COMMIT` + `IF EXISTS`/`IF NOT EXISTS` + CASCADE para dependencias.

## Tablas y su shape ACTUAL (post-Fase-C)

Las DDL originales (000007, 000034, etc.) declaraban `organization_id NOT NULL REFERENCES organizations(id)`. Esas columnas YA fueron dropeadas en Fase C (000141/142) y la tabla `organizations` ya no existe (000143). Además 000120 agregó una columna `status` a todas. Por eso el `down.sql` recrea el shape **post-Fase-C** (sin `organization_id`, con `status`), NO el shape literal de la migration original — recrear el original referenciaría `organizations(id)`, que no existe, y rompería el rollback.

## DDL del drop (up — orden importa)

```sql
BEGIN;

-- 1. Limpiar FK ENTRANTES a sessions ANTES de dropear sessions.
--    captured_prompts.session_id y verifications.session_id (ON DELETE SET NULL).
--    Se dropea la FK constraint EXPLÍCITAMENTE (no se depende del CASCADE
--    silencioso) y luego la columna; ambas tablas en 0 rows → seguro.
ALTER TABLE IF EXISTS captured_prompts DROP CONSTRAINT IF EXISTS captured_prompts_session_id_fkey;
ALTER TABLE IF EXISTS captured_prompts DROP COLUMN     IF EXISTS session_id;
ALTER TABLE IF EXISTS verifications    DROP CONSTRAINT IF EXISTS verifications_session_id_fkey;
ALTER TABLE IF EXISTS verifications    DROP COLUMN     IF EXISTS session_id;

-- 2. Drop de las 8 tablas. CASCADE remueve índices/triggers/policies/views/secuencias
--    dependientes (entity_state_transitions trae v_stuck_entities + trigger + función;
--    sessions trae RLS sessions_org_isolation de 000085).
--    saga_compensation_log NO tiene dependientes entrantes → sin CASCADE.
DROP TABLE IF EXISTS sessions                 CASCADE;
DROP TABLE IF EXISTS model_registry           CASCADE;
DROP TABLE IF EXISTS entity_state_transitions CASCADE;
DROP TABLE IF EXISTS system_state             CASCADE;
DROP TABLE IF EXISTS saga_compensation_log;
DROP TABLE IF EXISTS runtime_configs          CASCADE;
DROP TABLE IF EXISTS dead_letter_queue        CASCADE;
DROP TABLE IF EXISTS idempotency_keys         CASCADE;

-- 3. Limpiar la función append-only huérfana (CASCADE de la tabla ya la baja,
--    pero por las dudas si quedó referenciada).
DROP FUNCTION IF EXISTS entity_state_transitions_immutable() CASCADE;

COMMIT;
```

> **Decisión `sessions` (FK entrantes):** se limpian en el MISMO deploy. `sessions` es legacy y duplica a `auth_sessions` (la tabla de sesiones viva); las columnas `session_id` de `captured_prompts` y `verifications` colgaban de ella (FK ON DELETE SET NULL, 0 rows). Para poder dropear `sessions` se hace primero `DROP CONSTRAINT` + `DROP COLUMN` de ambas, sin depender del CASCADE silencioso. El `down` recrea las columnas nullable con su FK.

> **Decisión `sabotage_records`:** ya NO se dropea. Se preserva como `tdd_sabotage_records` (rename en 42.5). En su lugar entra `saga_compensation_log`, que comparte el cluster saga/infra (FK `run_id → flow_runs`, sin dependientes entrantes).

**Por qué CASCADE:** `entity_state_transitions` tiene una VIEW (`v_stuck_entities`), un TRIGGER append-only y una FUNCTION; `sessions` tiene una RLS policy (`sessions_org_isolation`, migration 000085). CASCADE las baja con la tabla sin tener que enumerarlas (más robusto ante drift de schema).

**Por qué el orden:** las FK entrantes a `sessions` (`captured_prompts`, `verifications`) deben limpiarse antes; el resto no tiene dependientes entrantes (solo FK salientes, que desaparecen con su propia tabla).

## Código a remover / refactorizar (resumen; detalle en tasks.md)

| Tabla | Acción de código | Compila sin tocar código? |
|---|---|---|
| `entity_state_transitions` | borrar pkg `lifecycletrack` (muerto, sin importers) | **sí** (nada lo importa) |
| `system_state` | refactor `orphan_runs_audit.go` (cursor `last_ack_at` → eliminar persistido o mover a `flow_config`) | no |
| `saga_compensation_log` | quitar/refactor SQL en `flow/saga.go` y `scheduler/cron/system/heartbeat_watcher.go` (FK saliente → flow_runs; nada la referencia) | sí (drop limpio) |
| `model_registry` | stubear `ModelExists`→true / quitar `validateModel`+`ErrModelUnknown`; mover pricing de `llm/registry` a constantes; borrar seeder + 2 Register | no |
| `runtime_configs` | eliminar feature hot-reload: borrar pkg `runtimeconfig`, wiring en main.go (Registry/Refresh/cron/SIGHUP), handler + rutas | no |
| `dead_letter_queue` | borrar `flow.DLQStore`, campo `DLQ` del runner + push en path de fallo, handler + rutas | no |
| `idempotency_keys` | borrar/stubear middleware `idempotency.go`, quitar `idempMW` de la cadena (main.go) + header CORS | no |
| `sessions` | borrar pkg `session` + handler + MCP tools; REFACTOR (no borrar) `timeline`/`search`/`lifecycle`/`stitcher`; quitar `session_id` de repos de `captured_prompts`/`verifications` | no |

## TDD plan

1. **Red**: test de migration roundtrip (`up` luego `down` luego `up`) — falla porque la tabla todavía existe / el down no recrea el shape.
2. **Green**: escribir `000149` up+down. Roundtrip pasa.
3. **Red**: `go build ./...` falla por referencias a tablas dropeadas (esperado).
4. **Green**: remover/refactorizar el código (tasks.md). Build verde.
5. **Red (boot)**: test de smoke que el binario bootea sin construir `runtimeconfig.Registry`/`idempMW`/`DLQStore`.
6. **Green**: wiring removido en main.go. Smoke pasa.
7. **Sabotaje** (sección obligatoria en tasks.md): re-introducir un `SELECT FROM runtime_configs` y confirmar que el test de boot/integración FALLA (anti-falso-positivo: prueba que el test realmente detecta la dependencia muerta).

## Riesgos y mitigaciones

| Riesgo | Mitigación |
|---|---|
| Dropear la tabla antes de remover el código → binario en prod crashea al boot/request | **Code-first**: build/vet/test verdes ANTES de aplicar la migration (orden de deploy documentado) |
| `sessions` es feature completa, romper `timeline`/`search`/`lifecycle`/`stitcher` que son KEEP | REFACTOR esos paquetes (no borrar): devolver vacío/0 donde leían `sessions`; tests de esos pkg deben seguir verdes |
| `system_state` guarda el cursor `last_ack_at` del `OrphanAuditor` | Refactor del cursor (eliminar persistido o mover a `flow_config`); si se mueve, hacerlo en la MISMA HU |
| `model_registry` gobierna creación de agentes (`ErrModelUnknown`) y pricing | Stubear `ModelExists`→true y mover pricing a constantes en código; sin esto, NO se pueden crear agentes |
| FK entrantes a `sessions` bloquean el DROP | `ALTER TABLE ... DROP CONSTRAINT ... _session_id_fkey` + `DROP COLUMN session_id` en `captured_prompts` y `verifications` ANTES del `DROP TABLE sessions` (explícito, no CASCADE silencioso) |
| Rollback (`down`) no restaura datos | Las 8 tablas tienen 0 filas → no hay datos que perder. `down` recrea el shape post-Fase-C para consistencia de migraciones; restore real de datos = pgBackRest |
| `down` recrea con `organization_id` → falla (organizations no existe) | El `down` recrea el shape **post-Fase-C** (sin `organization_id`), NO el DDL original |
| Linter referencia tabla inexistente (`dbconvlint` `commonNonPluralAllowed` tiene `model_registry`) | Quitar `model_registry` de la allowlist del linter (cosmético, pero evita ruido) |
| `down` recrea FK a `tasks`/`flow_runs`/`users` que podrían no existir | Esas tablas son KEEP (existen siempre post-Fase-C); las FK se recrean sin problema |

## Notas squawk / dbconvlint

- DROP sobre tablas con 0 filas: sin lock largo, sin reescritura → pasa squawk.
- `DROP COLUMN session_id`: en tabla vacía es instantáneo; squawk no marca rewrite en tablas vacías.
- `down` usa `CREATE TABLE IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS` y comentario `domain-lint-ignore-next: require-concurrent-index-creation` donde aplique (tabla nueva sin tráfico, igual que el original 000079).
- Quitar `model_registry` de `internal/dbconvlint/lint.go` `commonNonPluralAllowed` para que el linter no referencie una tabla que ya no existe.
