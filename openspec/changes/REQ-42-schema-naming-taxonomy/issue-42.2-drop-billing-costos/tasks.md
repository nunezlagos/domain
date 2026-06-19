# Tasks: issue-42.2-drop-billing-costos

## Verificación previa (bloqueante)

- [ ] Confirmar última migración aplicada = `000146`; `000147` reservada para rename (HU 42.1) → esta HU usa **`000148`**
- [ ] Confirmar 0 rows en `plans`, `budgets`, `cost_logs`, `cost_alerts_sent`, `org_cost_alert_thresholds` (introspección)
- [ ] Confirmar 0 FK entrantes hacia las 5 tablas; `cost_logs` solo tiene FKs **salientes** (`flow_runs`, `agent_runs`, `users`)
- [ ] Confirmar que ninguna tiene RLS policy (solo `otp_codes` y `audit_log` la tienen)
- [ ] Confirmar que `organizations.plan_id` ya cayó en `000143` (no queda FK viva a `plans`)
- [ ] `grep -rn "INSERT INTO cost_logs" internal/` → solo en tests (no hay writer de producción)

## Migración (000148)

- [ ] Crear `000148_drop_billing_costos.up.sql` con header completo (migration/author/issue/description/breaking/estimated_duration)
- [ ] `up`: `BEGIN; DROP TABLE IF EXISTS cost_logs; ... ; DROP TABLE IF EXISTS plans; COMMIT;` (orden: cost_logs primero, luego cost_alerts_sent, org_cost_alert_thresholds, budgets, plans)
- [ ] Crear `000148_drop_billing_costos.down.sql` que recrea **esqueleto mínimo** (estructura sin datos) y documenta que los datos NO se restauran
- [ ] Verificar `squawk` sobre ambos archivos (precedente aceptado: `000143`)
- [ ] Test roundtrip golang-migrate: `up → down → up` en DB fresca pasa sin error
- [ ] snake_case plural, `IF EXISTS`/`IF NOT EXISTS`, par up+down

## Código Go a eliminar — `plans`

- [ ] `internal/service/billing/service.go`: extirpar struct `Plan`, `CheckLimit`, `GetPlan`, `ErrPlanNotFound` y todas las queries `FROM plans`. **NO borrar el archivo** (mezcla `usage_counters` que se conserva)
- [ ] `internal/runner/agent/runner.go`: quitar campo `Billing *billing.Service` (l.76) y las llamadas `IncrementTokens`/`IncrementRuns` (l.309-310)
- [ ] `internal/seeds/plans_seeder.go`: **borrar archivo completo**
- [ ] `internal/seeds/seeds.go`: quitar referencias a `plans` en comentarios/orden (l.43, 47) — solo doc, no rompe build
- [ ] `cmd/domain/main.go`: quitar `seedRegistry.Register(&seeds.PlansSeeder{})` (l.485)
- [ ] `cmd/domain/install_cli.go`: quitar `registry.Register(&seeds.PlansSeeder{})` (l.1135)
- [ ] `internal/api/handler/api.go`: revisar inyección de `billing.Service` en `AppServer` y rutas de billing/plan si existen
- [ ] `cmd/domain-mcp/main.go`: revisar import/uso de `service/billing`
- [ ] `internal/cache/distributed/cache.go`: comentario `plans+custom_limits` (l.5) — cosmético

## Código Go a eliminar — `budgets`

- [ ] `internal/service/cost/analytics.go`: funciones/queries de budgets (l.246-299): listar/crear/checar budget vs spend
- [ ] `internal/api/handler/costanalytics.go`: handlers de budgets (l.108-145) y su registro en rutas
- [ ] `internal/api/handler/api.go`: rutas `/api/v1/cost/budgets` y wiring (l.415-417)
- [ ] `internal/mcp/server/resilience.go`: lógica MCP que consulta budgets (l.98, 127, 215, 232)
- [ ] `services/domain-admin/template/src/app/views/admin-maintainers/maintainer-registry.ts`: quitar entry `cost-budgets` (title `Cost Budgets`, apunta a `/api/v1/cost/budgets`, l.361-363)
- [ ] `services/domain-admin/template/src/app/views/admin-cost/routes.ts`: quitar mención `budgets` en description (l.11) — cosmético

## Código Go a eliminar — `cost_logs`

- [ ] `internal/service/usage/service.go`: quitar `SUM(...) FROM cost_logs` en `Current()` (l.180-184) e `History()` (CTE `cost`, l.247) — devolver 0 o eliminar columnas `CostUSD`/`TokensIn`/`TokensOut`. **Pkg KEEP, refactorizar no borrar**
- [ ] `internal/service/cost/analytics.go`: todas las queries `FROM cost_logs` (Spend l.45, breakdowns l.82-285)
- [ ] `internal/service/cost/service.go`: `DailyByOrg`/`DailyByAgent` que agregan `cost_logs` (l.42, 74)
- [ ] `internal/service/usagealerts/threshold_checker.go`: query de spend `FROM cost_logs` (l.32-68)
- [ ] `internal/admin/runners_usage/queries.go`: queries `FROM cost_logs` (l.211-227) y `categorize.go`
- [ ] `internal/api/handler/org_overview.go`: agregados `cost_logs` (l.214-248)
- [ ] `internal/api/handler/costanalytics.go`: endpoints que exponen los agregados de `cost_logs`
- [ ] `internal/dbconvlint/lint.go`: comentario de ejemplo `cost_logs usa recorded_at` (l.264) — cosmético

## Código Go a eliminar — `cost_alerts_sent`

- [ ] `internal/service/usagealerts/threshold_checker.go`: lógica de dedup que lee/inserta `cost_alerts_sent` (l.94-99)

## Código Go a eliminar — `org_cost_alert_thresholds`

- [ ] `internal/service/usagealerts/threshold_checker.go`: queries `SELECT`/`INSERT org_cost_alert_thresholds` (l.31-191)
- [ ] Si cae **todo** el cluster cost (cost_logs + cost_alerts_sent + org_cost_alert_thresholds), evaluar **borrar `threshold_checker.go` completo** y su wiring en `usagealerts/service.go` + `handler/usagealerts.go` + `main.go`

## Tests

- [ ] Test de migración: roundtrip `up→down→up` en DB fresca pasa
- [ ] Test: tras `up`, `SELECT ... FROM pg_tables WHERE tablename IN (...)` devuelve 0 filas
- [ ] `go build ./...` OK tras extirpar el código (red→green)
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./...` verde (ojo: tests que hacían `INSERT INTO cost_logs` deben borrarse o adaptarse)
- [ ] Frontend: `npm run build` OK tras quitar entry `cost-budgets`

## Sabotaje (anti-falsos positivos)

El falso positivo que más tememos: declarar "drop completo" cuando QUEDÓ una query SQL viva contra una tabla dropeada (el build podría compilar si la query está en un string sin tipado, y reventar en runtime al ejecutar `SELECT ... FROM cost_logs` contra una tabla inexistente → `ERROR: relation "cost_logs" does not exist`).

**Test de sabotaje (obligatorio):**

1. Tras completar el refactor, agregar a CI/local un grep guardián:
   ```bash
   # debe devolver CERO líneas (excluyendo tests muertos y comentarios)
   grep -rnE "(FROM|INTO|UPDATE|JOIN)[[:space:]]+(plans|budgets|cost_logs|cost_alerts_sent|org_cost_alert_thresholds)\b" \
     services/domain-backend/internal services/domain-backend/cmd \
     | grep -v "_test.go" | grep -vE "^\s*--|//"
   ```
2. **Romper a propósito:** reintroducir en `internal/service/usage/service.go` una query `SELECT SUM(cost_usd) FROM cost_logs ...`.
3. **Confirmar que el guardián FALLA:** el grep devuelve esa línea (rojo). Si el grep sigue en verde con la query reintroducida, el guardián es un FALSO POSITIVO y hay que arreglarlo.
4. **Confirmar runtime:** aplicar `000148 up` en una DB de prueba y ejecutar el endpoint que llamaba a esa query → debe devolver el comportamiento esperado (totales en 0 / columna ausente), NO `relation "cost_logs" does not exist`.
5. **Restaurar el fix:** quitar la query reintroducida → grep en verde, build OK, test pasa.

## Cierre

- [ ] `go vet ./...` sin warnings
- [ ] `go build ./...` OK
- [ ] `go test ./...` verde
- [ ] Grep guardián en verde (cero queries vivas contra las 5 tablas)
- [ ] Migración aplicada en VPS y verificada (`pg_tables` → 0 filas billing)
- [ ] Commit en rama `services`: `chore(req-42.2): drop dominio billing/costos y extirpar código (000148)`
- [ ] NO `git push` (repo local-only)
