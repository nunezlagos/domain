# issue-42.2-drop-billing-costos

**Origen:** `REQ-42-schema-naming-taxonomy`
**Prioridad tentativa:** alta
**Tipo:** chore (destructivo / data lifecycle)

## Historia de usuario

**Como** arquitecto del producto single-org sin facturación
**Quiero** eliminar por completo el dominio de billing/costos (`plans`, `budgets`, `cost_logs`, `cost_alerts_sent`, `org_cost_alert_thresholds`) junto con el código Go que las consulta
**Para** que el schema solo contenga tablas con un grupo/prefijo vigente, sin un cluster de facturación muerto que confunde la taxonomía y arrastra enforcement de cuotas que ya no aplica (la observabilidad de uso queda en `usage_*`)

## Contexto

El producto es single-org y NO factura. El dominio billing/costos quedó como residuo del diseño multi-tenant original:

- `plans` (creada en `000032`): catálogo de planes con `slug` unique y `soft_limit_ratio`. Sin FKs entrantes. La columna `organizations.plan_id` que la referenciaba ya fue dropeada en `000143`.
- `budgets` (creada en `000081`): presupuestos `amount_usd`/`period`/`warning_threshold_pct`. Sin FKs.
- `cost_logs` (creada en `000020`): costo por provider/model/run. FKs **salientes** a `flow_runs`, `agent_runs`, `users` (`ON DELETE SET NULL`); **ningún** dependiente entrante. No tiene writer de producción (el único `INSERT INTO cost_logs` vive en tests) — la tabla nunca se llena en runtime actual.
- `cost_alerts_sent` (creada en `000088`): dedup de alertas de costo por `alert_date`. Sin FKs.
- `org_cost_alert_thresholds` (creada en `000088`): umbrales de alerta de costo (prefijo `org_` legacy). Sin FKs.

Las 5 tablas tienen **0 rows** en el VPS (verificado en introspección). Ninguna tiene RLS policies. La observabilidad de producto se conserva en `usage_counters` / `usage_alerts` / `usage_alert_fires` (grupo `usage_`, fuera del scope de este drop).

## Criterios de aceptación

```gherkin
Feature: DROP del dominio billing/costos

  Background:
    Given el schema contiene las tablas plans, budgets, cost_logs, cost_alerts_sent y org_cost_alert_thresholds
    And ninguna tiene RLS policy ni FK entrante
    And la última migración aplicada es 000146

  Scenario: La migración 000148 dropea las 5 tablas billing
    When aplico la migración 000148 up
    Then plans, budgets, cost_logs, cost_alerts_sent y org_cost_alert_thresholds dejan de existir
    And las sequences asociadas (cost_logs_id_seq, cost_alerts_sent_id_seq, org_cost_alert_thresholds_id_seq) se eliminan con CASCADE
    And ninguna otra tabla pierde columnas ni FKs

  Scenario: El orden de DROP respeta dependencias
    Given cost_logs tiene FKs SALIENTES a flow_runs, agent_runs y users
    And NINGUNA tabla apunta a las tablas billing (cero FK entrantes)
    When aplico la migración 000148 up
    Then los DROP TABLE no fallan por dependencias
    And no hace falta CASCADE para borrar dependientes inexistentes

  Scenario: La migración es idempotente
    Given la migración 000148 ya se aplicó una vez
    When la vuelvo a aplicar
    Then los DROP TABLE IF EXISTS son no-op y no fallan

  Scenario: El down documenta que NO se recrea con datos
    When aplico la migración 000148 down
    Then se recrea el esqueleto mínimo de las 5 tablas (estructura, sin seeds ni datos)
    And el down deja constancia de que los datos billing NO se restauran (se perdieron en el drop)
    And el roundtrip up→down→up de golang-migrate queda consistente

  Scenario: El backend compila tras extirpar el código billing
    Given el paquete service/billing MEZCLA plans (drop) con usage_counters (keep)
    When elimino la lógica de plans y los callers de quota enforcement
    Then go build ./... compila sin referencias a las tablas dropeadas
    And el agent runner ya no llama Billing.IncrementTokens/IncrementRuns sobre plans
    And el cost-analytics (budgets/cost_logs) y el usagealerts/threshold_checker quedan sin queries a tablas inexistentes

  Scenario: Sabotaje — detectar drop incompleto
    Given un grep de "FROM plans", "FROM budgets", "FROM cost_logs", "FROM cost_alerts_sent", "FROM org_cost_alert_thresholds" sobre internal/
    When ejecuto el grep tras el refactor
    Then NO hay ninguna query SQL viva contra las tablas dropeadas (solo comentarios o tests marcados como muertos)
```

## Tablas en scope (SOLO billing del JSON de investigación)

| Tabla | Creada en | FK entrantes | FK salientes | RLS | Rows VPS | Acción |
|---|---|---|---|---|---|---|
| `plans` | `000032` | ninguna | ninguna | no | 0 | DROP |
| `budgets` | `000081` | ninguna | ninguna | no | 0 | DROP |
| `cost_logs` | `000020` | ninguna | `flow_runs`, `agent_runs`, `users` | no | 0 | DROP |
| `cost_alerts_sent` | `000088` | ninguna | ninguna | no | 0 | DROP |
| `org_cost_alert_thresholds` | `000088` | ninguna | ninguna | no | 0 | DROP |

> Fuera de scope: `sessions`, `model_registry`, `runtime_configs`, `dead_letter_queue`, `idempotency_keys`, `system_state`, `entity_state_transitions`, `sabotage_records`, `saga_compensation_log` — son otros clusters (legacy/infra) con su propia HU.

## Análisis breve

- **Qué pide realmente:** una migración destructiva (`000148`) que dropea las 5 tablas billing en una transacción, más la extirpación del código Go que las consulta para que el build siga compilando.
- **Riesgo en datos:** NULO. Las 5 tablas tienen 0 rows y no hay FK entrantes; ningún dato real se pierde.
- **Riesgo en compilación:** ALTO si se dropea sin tocar el código. `billing/service.go` MEZCLA `plans` (drop) con `usage_counters` (keep) — NO borrar el archivo entero, solo extirpar la parte de plans. El `cost-analytics` y el `threshold_checker` quedan rotos si no se quitan sus queries.
- **Decisión arquitectónica clave:** se ELIMINA el enforcement de cuotas por plan. El runner deja de llamar `Billing.IncrementTokens/IncrementRuns`. La observabilidad de uso pasa a depender solo de `usage_*`.
- **Esfuerzo tentativo:** M (la migración es trivial; el refactor del código billing/cost/usagealerts es lo que pesa).

## Verificación previa

- [ ] Confirmar que la última migración aplicada es `000146` y que `000147` (rename HU 42.1) ya está reservada → esta HU usa `000148`
- [ ] Confirmar 0 rows en las 5 tablas en el VPS (introspección: `plans=0, budgets=0, cost_logs=0, cost_alerts_sent=0, org_cost_alert_thresholds=0`)
- [ ] Confirmar que NINGUNA tabla tiene FK entrante hacia las 5 billing (introspección FK: solo FKs salientes de `cost_logs`)
- [ ] Confirmar que ninguna de las 5 tiene RLS policy (solo `otp_codes` y `audit_log` tienen RLS)
- [ ] Confirmar que `organizations.plan_id` ya fue dropeada en `000143` (no queda FK viva a `plans`)
- [ ] Verificar que `billing/service.go` separa claramente `plans` de `usage_counters` antes de extirpar
- [ ] `grep -rn "INSERT INTO cost_logs" internal/` → confirmar que solo aparece en tests (no hay writer de producción)
- [ ] Listar los 2 puntos de registro del `PlansSeeder` (`cmd/domain/main.go:485`, `cmd/domain/install_cli.go:1135`)
- [ ] Confirmar que la migración pasa `squawk` (DROP TABLE en single statement, igual que el precedente `000143`)

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
