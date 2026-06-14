# Design: issue-33.3-max-flow-duration-per-org

## Contexto

El `flowrunner.Runner` ejecuta flow_runs en goroutines. Si un flow
entra en loop infinito (e.g. un agente que sigue pidiendo tools),
la goroutine queda ocupada indefinidamente. Con 50 clientes
concurrentes, el scheduler se puede saturar.

Hoy el `RunRecovery` (issue-09.6) marca stale runs (sin update
por 5min) como failed. Es un fix global, no per-org. La mejora:
budget per-org, configurable, default 5min.

## Decisión arquitectónica

**Estrategia:** tabla `org_flow_config` con budget per-org +
extensión del `RunRecovery` para usarlo.

1. **Tabla `org_flow_config`:**
   ```sql
   CREATE TABLE org_flow_config (
     organization_id UUID PRIMARY KEY REFERENCES organizations(id),
     max_flow_duration_seconds INT NOT NULL DEFAULT 300
       CHECK (max_flow_duration_seconds >= 10 AND max_flow_duration_seconds <= 86400),
     updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
   );
   ```
   Constraint: 10s mínimo (evitar 0 que cancelaría
   instantáneamente), 86400s = 24h máximo.

2. **Seeder** que inserta default 300s para cada org existente
   (idempotente, INSERT ON CONFLICT DO NOTHING).

3. **Hot-reload via `runtimeconfig.Registry`** (issue-27.3):
   refresca la config cada 30s. El cache in-memory se actualiza.

4. **Extensión del `RunRecovery`** en `cmd/domain/main.go`:
   ```go
   func (r *Runner) RunRecovery(ctx, cfg RecoveryConfig) {
     ticker := time.NewTicker(cfg.PollInterval)
     for {
       select {
       case <-ctx.Done(): return
       case <-ticker.C:
         // 1. Buscar flow_runs en estado "running" con
         //    started_at < NOW() - max_duration_per_org
         // 2. Cancelar (UPDATE flow_runs SET status='failed', ...)
         // 3. Loggear métrica
       }
     }
   }
   ```
   Query:
   ```sql
   SELECT fr.id, fr.organization_id, fr.started_at, ofc.max_flow_duration_seconds
   FROM flow_runs fr
   JOIN org_flow_config ofc ON ofc.organization_id = fr.organization_id
   WHERE fr.status = 'running'
     AND fr.started_at < NOW() - (ofc.max_flow_duration_seconds * INTERVAL '1 second')
   ```

5. **Cancelación real del context:** además de marcar el flow_run
   como failed en DB, propagar el cancel via el `flowRunCancel`
   map que el runner mantiene (issue-09.6 ya tiene este patrón).
   ```go
   if cancel, ok := r.activeFlowCancels[runID]; ok {
     cancel()  // context.CancelFunc
   }
   ```

6. **Métrica:** `metrics.FlowRunCancelledByMaxDuration.WithLabelValues(orgID).Inc()`.

7. **Admin endpoint** `PUT /api/v1/admin/flow-config/{orgID}`:
   body `{max_flow_duration_seconds: 60}`. Validar rango
   10-86400. UPSERT en `org_flow_config`. Audit log.

8. **Comportamiento con sub-flows:** el cancel del top-level
   propaga via context. Los sub-flows (que también tienen su
   context derivado) heredan el cancel. El
   `flow.SignalStore` y `flow.DLQ` ya manejan esto.

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Timeout por flow_run individual (no per-org) | Demasiado granular: el user tendría que setear timeout en cada flow. Per-org es lo operacional. |
| B | Cancelación cooperativa (el flow chequea el budget) | No funciona con código third-party (Llamadas HTTP, sleep, etc). Hard cancel via context es más robusto. |
| C | Hard cap con reject de nuevos flows cuando budget exhausted | El user fue explícito: no paywall. Soft budget + cancel existente. |
| D | Por step (no por flow total) | Más complejo, menos útil operativamente. El flow total es la unidad. |

## Por qué tabla + extensión del RunRecovery gana

- **Reutiliza infra existente:** `RunRecovery` ya tiene el loop,
  el cancel map, las métricas. Solo se agrega la query per-org.
- **Hot-reload:** cambiar el budget no requiere deploy.
- **Hard cancel:** context.CancelFunc es la forma robusta de
  parar goroutines, incluso si el código no es cooperativo.
- **Auditable:** el `failed` con `error_code: "max_duration_exceeded"`
  queda en `flow_runs` y `audit_log`.

## Detalle de implementación

- Migración: `migrations/000094_org_flow_config.sql`.
- Seeder: `internal/seeds/org_flow_config_seeder.go`.
- `internal/service/flow/budget.go`:
  - `GetMaxDuration(ctx, pool, orgID) (time.Duration, error)`.
  - Cache in-memory 30s con hot-reload via runtime config.
- Modificar `flowrunner.RunRecovery` (cmd/domain/main.go) para
  usar la query con JOIN.
- Admin endpoint: `internal/api/handler/admin/flow_config.go`.

## Riesgos

- **R1:** Cancelar un flow a la fuerza puede dejar recursos
  inconsistent (e.g. file lock, external API call started).
  **Mitigación:** documentar que flows que mutan estado externo
  deben usar sagas o compensaciones. No es problema de este
  issue.
- **R2:** El flow_run cancelado NO se reintenta. **Decisión del
  user:** aceptable, es "hard cancel". Si quiere retry, lo hace
  manual via `domain flows retry <id>`.
- **R3:** RunRecovery corre cada 60s. Un flow puede vivir 60s
  más de su budget antes de ser cancelado. **Aceptable:** no
  crítico. Si urge, ajustar el PollInterval a 10s.
