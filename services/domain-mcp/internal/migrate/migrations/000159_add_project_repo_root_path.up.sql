-- migration: add_project_repo_root_path
-- author: mnunez@saargo.com
-- issue: multi-repo dashboard — folder por remoto
-- description: root_path por remoto git en project_repositories. Indica en qué
--   carpeta del checkout vive ese repo (ej. '/' o '/domain/services/'). Opcional
--   (NULL = sin especificar / raíz). Lo consume el dashboard (mantenedor de
--   proyectos) y el LLM para ubicar el código de cada remoto en un monorepo o
--   en un layout multi-repo.
-- breaking: false
-- estimated_duration: <1s (ADD COLUMN nullable, sin default ni reescritura)

ALTER TABLE project_repositories ADD COLUMN root_path VARCHAR(500);
