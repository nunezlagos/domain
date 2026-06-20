-- issue-21.6: drop org_delete_log.
-- Era el audit del hard-delete de organizaciones (DeleteService), removido en
-- issue-21.5 (single-org). Sin consumidores Go restantes.
DROP TABLE IF EXISTS org_delete_log;
