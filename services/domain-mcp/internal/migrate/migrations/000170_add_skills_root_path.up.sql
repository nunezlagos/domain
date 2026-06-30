-- migration: 000170_add_skills_root_path
-- author: NunezLagos
-- issue: legacy
-- estimated_duration: unknown
-- description: root_path en skills para vincular skills de stack a un subpath
--   del repo (monorepo/submódulos). NULL = aplica a todo el proyecto/root.
-- breaking: no (columna nullable, sin backfill)

ALTER TABLE skills ADD COLUMN IF NOT EXISTS root_path TEXT;
