-- down de 000291_compliance_marcos_normativos
--
-- Las tablas por proyecto se dropean primero por las FK al catálogo. current_project_id() NO se
-- dropea: la crean también la 000287 y la 000288, y sacarla acá rompería el RLS de knowledge_docs,
-- knowledge_chunks, webhooks y webhook_deliveries.

DROP TABLE IF EXISTS project_control_status CASCADE;
DROP TABLE IF EXISTS project_compliance_frameworks CASCADE;
DROP TABLE IF EXISTS framework_controls CASCADE;
DROP TABLE IF EXISTS compliance_controls CASCADE;
DROP TABLE IF EXISTS compliance_frameworks CASCADE;
