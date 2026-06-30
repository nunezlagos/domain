-- migration: 000159_add_project_repo_root_path
-- author: NunezLagos
-- issue: legacy
-- description: columna root_path en project_repositories
-- breaking: no
-- estimated_duration: unknown

ALTER TABLE project_repositories ADD COLUMN root_path VARCHAR(500);
