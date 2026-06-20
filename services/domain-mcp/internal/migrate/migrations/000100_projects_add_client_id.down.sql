DROP TRIGGER IF EXISTS projects_client_same_org_check ON projects;
DROP FUNCTION IF EXISTS projects_check_client_same_org();
DROP INDEX IF EXISTS projects_client_id_idx;
ALTER TABLE projects DROP COLUMN IF EXISTS client_id;
