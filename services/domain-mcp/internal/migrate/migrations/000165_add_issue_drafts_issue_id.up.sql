-- migration: add_issue_drafts_issue_id
-- author: mnunez@saargo.com
-- issue: materializacion del wizard — vincular draft committeado con su issue real
-- description: agrega issue_id (nullable) a issue_drafts para enlazar el draft
--   con el issue materializado en su Commit. Antes el Commit solo marcaba
--   status=committed y nunca escribia en issues/sdd_requirements (tablas
--   vacias). Ahora el Commit crea el issue real y guarda su id aca para
--   trazabilidad (draft -> issue). SET NULL: borrar el issue no borra el draft,
--   solo desvincula. Tabla greenfield (0 filas), lock instantaneo.
-- breaking: false
-- estimated_duration: <1s (ADD COLUMN nullable, sin reescritura)

ALTER TABLE issue_drafts ADD COLUMN issue_id UUID REFERENCES issues(id) ON DELETE SET NULL;

-- Postgres NO indexa FKs automaticamente; el ON DELETE SET NULL haria seq-scan
-- de issue_drafts al borrar un issue. Indice de cobertura (consistente con la
-- convencion de 000161 que indexa cada FK que agrega).
-- domain-lint-ignore-next: require-concurrent-index
CREATE INDEX IF NOT EXISTS issue_drafts_issue_id_idx ON issue_drafts (issue_id);
