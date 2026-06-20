-- migration: drop_org_id_satellites
-- author: mnunez@saargo.com
-- issue: REQ-21.6 (Fase C — destructiva, irreversible sin restore)
-- description: DROP COLUMN organization_id en las 5 satélites que fueron
--   desacopladas per-consumer en Fase B (commit bcc5196, migraciones 000135-139):
--   - cost_alerts_sent (nullable, UNIQUE(alert_date))
--   - org_cost_alert_thresholds (id BIGSERIAL PK, UNIQUE(org_id) — el col es redundante)
--   - org_flow_config (id BIGSERIAL PK, UNIQUE(org_id))
--   - usage_counters (id BIGSERIAL PK, UNIQUE(period_start))
--   - org_enrollment_tokens (singleton global activo)
--   Cada DROP COLUMN preserva las filas (PG no toca data al dropear columnas
--   no-PK). La constraint UNIQUE sobre organization_id en cost_alerts_sent
--   ya se removió en 000135; en org_cost_alert_thresholds/org_flow_config/
--   usage_counters/org_enrollment_tokens la constraint es UNIQUE, no PK, así
--   que DROP COLUMN funciona sin reordenar nada.
--   Pre-requisito: 000140 ejecutada (las FKs a organizations(id) están dropeadas).
-- breaking: true (datos organization_id se pierden — restore vía pgBackRest)
-- estimated_duration: <1s

ALTER TABLE cost_alerts_sent           DROP COLUMN IF EXISTS organization_id;
ALTER TABLE org_cost_alert_thresholds  DROP COLUMN IF EXISTS organization_id;
ALTER TABLE org_flow_config            DROP COLUMN IF EXISTS organization_id;
ALTER TABLE usage_counters             DROP COLUMN IF EXISTS organization_id;
ALTER TABLE org_enrollment_tokens      DROP COLUMN IF EXISTS organization_id;

-- Y de paso: el index que se creó sobre (organization_id) en cada uno también
-- desaparece automáticamente con la columna. No hace falta DROP INDEX explícito.
