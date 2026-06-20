ALTER TABLE IF EXISTS projects DROP CONSTRAINT IF EXISTS projects_template_id_fkey;
DROP TABLE IF EXISTS project_templates CASCADE;
