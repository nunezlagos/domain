# Design: issue-33.4-quota-snapshot-dashboard-ready

## Contexto

El cliente (user de org X) quiere ver "cuánto consumí hoy" sin
tener que pedirle al admin. Hoy los datos existen (cost_logs,
observations, etc.) pero no hay endpoint que los agregue y los
sirva.

El endpoint es read-only, scope propio de la org, y formatea
counters en un JSON consumible por un dashboard futuro.

## Decisión arquitectónica

**Estrategia:** 2 endpoints GET (`/usage/current` + `/usage/history`)
+ vista materializada para performance histórica.

1. **Endpoint `/api/v1/usage/current`:**
   - Auth: Bearer OR session (la org se saca del principal).
   - Query a la DB:
     ```sql
     SELECT
       (SELECT COUNT(*) FROM observations
        WHERE organization_id = $1
          AND created_at >= date_trunc('day', NOW() AT TIME ZONE 'UTC')
          AND deleted_at IS NULL) AS observations,
       (SELECT COUNT(*) FROM agents
        WHERE organization_id = $1
          AND deleted_at IS NULL) AS agents,
       (SELECT COUNT(*) FROM agent_runs
        WHERE organization_id = $1
          AND started_at >= date_trunc('day', NOW() AT TIME ZONE 'UTC')) AS agent_runs_today,
       (SELECT COUNT(*) FROM flow_runs
        WHERE organization_id = $1
          AND started_at >= date_trunc('day', NOW() AT TIME ZONE 'UTC')) AS flow_runs_today,
       (SELECT COALESCE(SUM(cost_usd), 0) FROM cost_logs
        WHERE organization_id = $1
          AND created_at >= date_trunc('day', NOW() AT TIME ZONE 'UTC')) AS cost_usd_today,
       (SELECT COALESCE(SUM(tokens_in), 0) FROM cost_logs
        WHERE organization_id = $1
          AND created_at >= date_trunc('day', NOW() AT TIME ZONE 'UTC')) AS tokens_in_today,
       (SELECT COALESCE(SUM(tokens_out), 0) FROM cost_logs
        WHERE organization_id = $1
          AND created_at >= date_trunc('day', NOW() AT TIME ZONE 'UTC')) AS tokens_out_today;
     ```
   - JOIN con `org_rate_limits` y `org_flow_config` para los
     `limits`.
   - Formatea JSON según el schema del issue.

2. **Endpoint `/api/v1/usage/history?days=N`:**
   - Validar: `1 <= days <= 365`, default 7.
   - Source: vista materializada `usage_daily_aggregates`
     (refresh nightly):
     ```sql
     CREATE MATERIALIZED VIEW usage_daily_aggregates AS
     SELECT
       organization_id,
       date_trunc('day', created_at AT TIME ZONE 'UTC')::DATE AS day,
       COUNT(*) FILTER (WHERE table_name = 'observations') AS observations,
       SUM(cost_usd) FILTER (WHERE table_name = 'cost_logs') AS cost_usd,
       ...
     GROUP BY organization_id, day;
     CREATE UNIQUE INDEX ON usage_daily_aggregates(organization_id, day);
     ```
   - Refresca con `REFRESH MATERIALIZED VIEW CONCURRENTLY` en el
     job de cost (issue-15.3) una vez por día.
   - Query:
     ```sql
     SELECT day, observations, cost_usd, agent_runs, flow_runs
     FROM usage_daily_aggregates
     WHERE organization_id = $1
       AND day >= CURRENT_DATE - INTERVAL '$N days'
     ORDER BY day DESC;
     ```

3. **Filtro de org:** el handler SIEMPRE usa
   `principal.OrganizationID` del context post-auth. NUNCA
   acepta `?org_id=X` como param. El cliente solo ve SU org.

4. **Performance:**
   - `/current` con 1M cost_logs: <100ms con índices correctos.
   - `/history` con materialized view: <50ms para 365 días.
   - Si la materialized view crece mucho (>10M filas),
     particionar por mes o trimestre.

5. **Estructura de archivos:**
   - `internal/api/handler/usage.go` con los 2 handlers.
   - Wire en `cmd/domain/main.go` con auth (allowlist para
     no-auth no aplica — todos los endpoints requieren auth).
   - OpenAPI annotations (32.3): tag `Usage`, summary "Current
     usage snapshot" / "Historical usage".

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Un solo endpoint `/usage` que retorna current + history | Más complejo en el schema, cliente no lo necesita. 2 endpoints claros. |
| B | Permitir `?org_id=X` para admins ver otras orgs | Out of scope: el admin tiene endpoints separados. Este es para el user final. |
| C | Streaming del log (SSE o WebSocket) | No aplica: es un snapshot, no un live feed. |
| D | GraphQL para queries flexibles | El server es REST. Agregar GraphQL es overkill. |

## Por qué 2 endpoints + materialized view gana

- **Simple:** 2 endpoints, contrato claro.
- **Performante:** la materialized view pre-calcula las
  agregaciones costosas. La query final es O(days) en vez de
  O(rows).
- **Seguro:** filtro de org SIEMPRE desde el principal. Cero
  param de org en el query string.
- **Extensible:** si en el futuro se quiere breakdown por
  proyecto, agente, etc., se agrega param `?breakdown=project`
  con la misma materialized view.

## Detalle de implementación

- Migración: `migrations/000095_usage_aggregates.sql` con la
  materialized view.
- `internal/api/handler/usage.go`:
  - `UsageCurrentGET(w, r)`.
  - `UsageHistoryGET(w, r)` con parsing de `?days=N`.
- Job nightly: agregar a `runUsageAlertEvaluator` (existente) un
  step `REFRESH MATERIALIZED VIEW CONCURRENTLY
  usage_daily_aggregates`.
- Tests: filtrar por org, formato JSON, performance.

## Riesgos

- **R1:** Materialized view lockea durante refresh. **Mitigación:**
  usar `CONCURRENTLY` (no requiere lock exclusivo). Requiere
  UNIQUE index (que ya está).
- **R2:** Endpoint `/current` ejecuta 7 sub-queries. **Aceptable:**
  <100ms con índices. Si urge, consolidar en 1 query con
  COUNT() FILTER.
- **R3:** User abusa del endpoint (scrapea cada segundo).
  **Mitigación:** el rate limit per-org de 33.1 aplica. 1000/min
  es más que suficiente para uso humano.
