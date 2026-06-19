-- migration: drop_billing_costos
-- author: mnunez@saargo.com
-- issue: REQ-42.2 (schema-naming-taxonomy — DROP dominio billing/costos)
-- description: DESTRUCTIVO. Dropea el cluster completo de billing/costos, que
--   quedó como residuo del diseño multi-tenant. El producto es single-org y
--   NO factura; el enforcement de cuotas por plan se elimina y la
--   observabilidad de uso queda en el grupo usage_* (usage_counters/
--   usage_alerts/usage_alert_fires, fuera de scope).
--
--   Tablas dropeadas (todas con 0 rows en el VPS, ninguna con RLS):
--     - cost_logs (creada 000020): costo por provider/model/run. FKs
--       SALIENTES a flow_runs/agent_runs/users; SIN dependientes entrantes.
--       Sin writer de producción (INSERT solo en tests) → siempre vacía.
--     - cost_alerts_sent (creada 000088): dedup de alertas de costo.
--     - org_cost_alert_thresholds (creada 000088, PK swap 000136): umbrales
--       de alerta de costo (prefijo org_ legacy).
--     - budgets (creada 000081): presupuestos de gasto LLM.
--     - plans (creada 000032): catálogo de planes. organizations.plan_id ya
--       cayó en 000143, así que no queda FK viva hacia plans.
--
--   Orden: cost_logs primero (única con FKs salientes vivas), luego el resto.
--   NINGUNA tiene FK entrante → ningún DROP falla por dependientes y NO se
--   necesita CASCADE. DROP TABLE elimina índices, constraints, triggers y las
--   sequences BIGSERIAL owned (cost_logs_id_seq, cost_alerts_sent_id_seq,
--   org_cost_alert_thresholds_id_seq).
--
--   Precedente de DROP atómico: 000143 (drop organizations). Idempotente vía
--   IF EXISTS. Pasa squawk (un statement por tabla, dentro de tx).
--
--   down: recrea esqueleto mínimo (estructura sin datos) para roundtrip
--   golang-migrate. NO restaura datos (se perdieron en este up).
-- breaking: true (las 5 tablas y sus datos se pierden — restore vía pgBackRest)
-- estimated_duration: <1s

BEGIN;

DROP TABLE IF EXISTS cost_logs;
DROP TABLE IF EXISTS cost_alerts_sent;
DROP TABLE IF EXISTS org_cost_alert_thresholds;
DROP TABLE IF EXISTS budgets;
DROP TABLE IF EXISTS plans;

COMMIT;
