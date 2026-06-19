# Design: issue-42.2-drop-billing-costos

## Decisión arquitectónica

**Drop atómico en una sola transacción, con extirpación quirúrgica del código.** Las 5 tablas billing se eliminan dentro de un `BEGIN/COMMIT` (mismo patrón que el rename atómico de `000146`). No hay datos que migrar (0 rows) ni FKs entrantes que romper, así que el `up` es una secuencia de `DROP TABLE IF EXISTS` ordenada por seguridad.

**Orden de DROP.** Como NINGUNA de las 5 tablas tiene FK entrante, el orden entre ellas es indiferente a nivel de constraints. Aun así dropeamos `cost_logs` primero por higiene (es la única con FKs salientes vivas a `flow_runs`/`agent_runs`/`users`; borrarla primero deja el grafo de dependencias más limpio para auditoría). No se usa `CASCADE` salvo donde haga falta limpiar sequences/índices que Postgres no borre solo — pero `DROP TABLE` ya elimina índices y constraints propios; las sequences `BIGSERIAL` (`cost_logs_id_seq`, `cost_alerts_sent_id_seq`, `org_cost_alert_thresholds_id_seq`) caen como objetos *owned* al dropear la tabla dueña.

**El enforcement de cuotas por plan se ELIMINA, no se reemplaza.** La decisión es que un producto single-org sin facturación no necesita gating por `plans`. El paquete `billing` se conserva SOLO en su parte de `usage_counters` (lectura para dashboard de uso); toda la lógica de `plans` (struct `Plan`, `CheckLimit`, `GetPlan`, `ErrPlanNotFound`) se extirpa. El runner deja de llamar `Billing.IncrementTokens/IncrementRuns`.

**`cost_logs` no tenía writer de producción.** El único `INSERT INTO cost_logs` vive en tests. Por eso los servicios que la LEEN (`usage`, `cost`, `usagealerts`, `org_overview`) siempre operaban sobre una tabla vacía: agregaban `SUM(cost_usd)=0`. Extirpar esas queries no cambia el comportamiento observable (los totales ya eran 0), solo elimina la dependencia de una tabla inexistente.

## DDL — migración 000148 up (DROP)

```sql
BEGIN;

-- Orden: cost_logs primero (única con FKs salientes vivas), luego el resto.
-- Ninguna tiene FK entrante → ningún DROP falla por dependientes.
-- DROP TABLE elimina índices, constraints y sequences BIGSERIAL owned.
DROP TABLE IF EXISTS cost_logs;
DROP TABLE IF EXISTS cost_alerts_sent;
DROP TABLE IF EXISTS org_cost_alert_thresholds;
DROP TABLE IF EXISTS budgets;
DROP TABLE IF EXISTS plans;

COMMIT;
```

## DDL — migración 000148 down (NO recrea datos)

El `down` recrea un **esqueleto mínimo** (estructura sin seeds ni datos) para que el roundtrip `up→down→up` de golang-migrate quede consistente en una DB fresca. **NO** restaura datos billing: se perdieron en el drop. En este producto las tablas estaban vacías, así que el esqueleto es puramente para satisfacer el reverse de la herramienta de migración. El esqueleto refleja el estado **actual** del schema (post `000141-143`): SIN `organization_id` (esa columna y su FK ya no existen).

```sql
-- Reverse: recrea esqueleto mínimo. NO restaura datos (se perdieron en el up).
CREATE TABLE IF NOT EXISTS plans (
  id BIGSERIAL PRIMARY KEY,
  slug VARCHAR(50) UNIQUE NOT NULL,
  ...
);
CREATE TABLE IF NOT EXISTS budgets (...);
CREATE TABLE IF NOT EXISTS cost_logs (...);  -- FKs salientes a flow_runs/agent_runs/users
CREATE TABLE IF NOT EXISTS cost_alerts_sent (...);
CREATE TABLE IF NOT EXISTS org_cost_alert_thresholds (...);
```

(El archivo de migración trae el DDL completo; aquí solo se documenta la intención.)

## Riesgos y mitigaciones

| Riesgo | Mitigación |
|---|---|
| Dropear `plans` sin tocar `billing/service.go` → NO compila | Extirpar struct `Plan`, `CheckLimit`, `GetPlan`, `ErrPlanNotFound`. NO borrar el archivo (mezcla `usage_counters` que se conserva) |
| `runner.go:309-310` sigue llamando `Billing.IncrementTokens/IncrementRuns` | Quitar el campo `Billing *billing.Service` (l.76) y las 2 llamadas; el runner ya no hace gating por plan |
| `PlansSeeder` registrado en 2 lugares → panic/build error si queda colgado | Borrar `plans_seeder.go` y quitar `Register(&seeds.PlansSeeder{})` de `main.go:485` e `install_cli.go:1135` |
| `cost-analytics` (budgets) deja rutas HTTP huérfanas | Quitar handlers de budgets (`costanalytics.go:108-145`), rutas (`api.go:415-417`) y lógica MCP (`resilience.go`) |
| `usage/service.go` LEE `cost_logs` (pkg KEEP) → no se puede borrar el pkg | Refactor in-place: quitar los `SUM(... ) FROM cost_logs` en `Current()` (l.180-184) e `History()` (l.247); devolver 0 o eliminar las columnas `CostUSD`/`TokensIn`/`TokensOut` del snapshot |
| `usagealerts/threshold_checker.go` consulta las 3 tablas de costo | Si cae todo el cluster cost, evaluar borrar `threshold_checker.go` completo y su wiring (`usagealerts/service.go`, `handler/usagealerts.go`, `main.go`) |
| Angular referencia rutas `/api/v1/cost/budgets` | Quitar entry `cost-budgets` de `maintainer-registry.ts:361-363` y la mención `budgets` en `admin-cost/routes.ts:11` |
| `squawk` marca `DROP TABLE` como riesgoso | Precedente aceptado: `000143` dropeó `organizations`. Single statement por tabla, `IF EXISTS`, dentro de tx |
| Roundtrip de golang-migrate falla sin down | El down recrea esqueleto mínimo (estructura, sin datos) |

## TDD plan

1. **Red:** test de migración roundtrip `000148 up → down → up` en DB fresca debe pasar (golang-migrate test helper o `migrate -path ... up/down`).
2. **Red:** `go build ./...` falla mientras queden referencias a `plans`/`budgets`/`cost_logs`/`cost_alerts_sent`/`org_cost_alert_thresholds`.
3. **Green:** extirpar el código billing/cost/usagealerts (ver `tasks.md`) hasta que compile.
4. **Refactor:** simplificar el paquete `billing` para que solo exponga lo de `usage_counters`.
5. **Sabotaje (anti-falso-positivo):** ver `tasks.md` — reintroducir temporalmente una query `FROM cost_logs` y confirmar que el grep guardián la detecta y el build la rechaza.

## Verificación post-aplicación (en VPS)

```sql
-- Las 5 tablas NO deben existir:
SELECT tablename FROM pg_tables
WHERE schemaname='public'
  AND tablename IN ('plans','budgets','cost_logs','cost_alerts_sent','org_cost_alert_thresholds');
-- esperado: 0 filas

-- Ninguna otra tabla perdió columnas/FKs (smoke):
SELECT count(*) FROM pg_constraint WHERE contype='f';  -- comparar con baseline pre-drop
```
