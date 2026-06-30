-- migration: 000148_drop_billing_costos
-- author: NunezLagos
-- issue: legacy
-- description: elimina las tablas de billing/costos en desuso (DROP TABLE)
-- breaking: yes
-- estimated_duration: unknown

BEGIN;

DROP TABLE IF EXISTS cost_logs;
DROP TABLE IF EXISTS cost_alerts_sent;
DROP TABLE IF EXISTS org_cost_alert_thresholds;
DROP TABLE IF EXISTS budgets;
DROP TABLE IF EXISTS plans;

COMMIT;
