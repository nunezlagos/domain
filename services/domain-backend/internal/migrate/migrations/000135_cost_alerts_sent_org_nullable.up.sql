-- migration: cost_alerts_sent_org_nullable
-- author: mnunez@saargo.com
-- issue: REQ-21.6 (Fase B — per-consumer cleanup)
-- description: cost_alerts_sent.organization_id deja de ser NOT NULL y la UNIQUE
--   constraint pasa de (organization_id, alert_date) a (alert_date) solamente.
--   El código de threshold_checker.SendAlerts ya NO escribe organization_id en
--   el INSERT (issue-21.5 single-org + REQ-21.6); la columna queda huérfana
--   → droppable en Fase C junto con la tabla organizations.
--   Mantiene anti-spam: solo 1 alerta por (alert_date).
-- breaking: false (código ya no usa la columna)
-- estimated_duration: <1s

ALTER TABLE cost_alerts_sent
  ALTER COLUMN organization_id DROP NOT NULL;

ALTER TABLE cost_alerts_sent
  DROP CONSTRAINT IF EXISTS cost_alerts_sent_organization_id_alert_date_key;

ALTER TABLE cost_alerts_sent
  ADD CONSTRAINT cost_alerts_sent_alert_date_key UNIQUE (alert_date);
