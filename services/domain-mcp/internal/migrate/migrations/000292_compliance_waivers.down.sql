-- down de 000292_compliance_waivers
--
-- current_project_id() NO se dropea: la crean también la 000287, 000288 y 000291, y sacarla acá
-- rompería el RLS de knowledge, webhooks y las tablas de compliance.

DROP TABLE IF EXISTS compliance_waivers CASCADE;
