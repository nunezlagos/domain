


















DROP FUNCTION IF EXISTS current_org_id() CASCADE;


DROP TRIGGER IF EXISTS projects_client_same_org_check ON projects;








ALTER TABLE IF EXISTS organizations DROP CONSTRAINT IF EXISTS organizations_plan_id_fkey;


DROP TABLE IF EXISTS organizations CASCADE;




DROP INDEX IF EXISTS projects_organization_id_unique;
