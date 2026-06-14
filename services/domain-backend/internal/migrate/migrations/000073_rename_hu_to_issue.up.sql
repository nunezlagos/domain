-- migration: rename_hu_to_issue
-- author: nunezlagos
-- issue: RFC 0007
-- description: rename user_stories→issues, hu_drafts→issue_drafts, hu_draft_steps_log→issue_draft_steps_log + columnas hu_id→issue_id en gherkin_scenarios/proposals/designs/tasks/code_references + committed_hu_id→committed_issue_id en intake_payloads
-- breaking: true
-- estimated_duration: <2s (renames son metadata-only en Postgres)

BEGIN;

-- 1) Tablas principales
ALTER TABLE user_stories RENAME TO issues;
ALTER TABLE hu_drafts RENAME TO issue_drafts;
ALTER TABLE hu_draft_steps_log RENAME TO issue_draft_steps_log;

-- 2) Columna draft_id en steps_log → issue_draft_id (más explícita post-rename)
ALTER TABLE issue_draft_steps_log RENAME COLUMN draft_id TO issue_draft_id;

-- 3) Columnas hu_id → issue_id (FKs siguen apuntando a 'issues' automáticamente)
ALTER TABLE gherkin_scenarios RENAME COLUMN hu_id TO issue_id;
ALTER TABLE proposals RENAME COLUMN hu_id TO issue_id;
ALTER TABLE designs RENAME COLUMN hu_id TO issue_id;
ALTER TABLE tasks RENAME COLUMN hu_id TO issue_id;
ALTER TABLE code_references RENAME COLUMN hu_id TO issue_id;
ALTER TABLE intake_payloads RENAME COLUMN committed_hu_id TO committed_issue_id;

-- 4) Triggers: renombrar set_updated_at_user_stories → set_updated_at_issues
DROP TRIGGER IF EXISTS set_updated_at_user_stories ON issues;
CREATE TRIGGER set_updated_at_issues
  BEFORE UPDATE ON issues
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

DROP TRIGGER IF EXISTS set_updated_at_hu_drafts ON issue_drafts;
CREATE TRIGGER set_updated_at_issue_drafts
  BEFORE UPDATE ON issue_drafts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 5) Índices que llevan el nombre tabla viejo (Postgres no los renombra solo)
ALTER INDEX IF EXISTS user_stories_pkey RENAME TO issues_pkey;
ALTER INDEX IF EXISTS user_stories_organization_id_slug_unique RENAME TO issues_organization_id_slug_unique;
ALTER INDEX IF EXISTS user_stories_organization_id_idx RENAME TO issues_organization_id_idx;
ALTER INDEX IF EXISTS user_stories_status_idx RENAME TO issues_status_idx;
ALTER INDEX IF EXISTS user_stories_req_id_idx RENAME TO issues_req_id_idx;

ALTER INDEX IF EXISTS hu_drafts_pkey RENAME TO issue_drafts_pkey;
ALTER INDEX IF EXISTS hu_drafts_created_by_idx RENAME TO issue_drafts_created_by_idx;
ALTER INDEX IF EXISTS hu_drafts_status_idx RENAME TO issue_drafts_status_idx;

ALTER INDEX IF EXISTS hu_draft_steps_log_pkey RENAME TO issue_draft_steps_log_pkey;
ALTER INDEX IF EXISTS hu_draft_steps_log_draft_id_idx RENAME TO issue_draft_steps_log_issue_draft_id_idx;

-- 6) entity_state_transitions: convertir entity_kind='hu' → 'issue'
UPDATE entity_state_transitions SET entity_kind = 'issue' WHERE entity_kind = 'hu';

-- 7) GRANT defensivo para las tablas renombradas (por si app_user no auto-hereda los grants)
DO $grants$
DECLARE
  tbl text;
BEGIN
  FOR tbl IN SELECT unnest(ARRAY['issues', 'issue_drafts', 'issue_draft_steps_log']) LOOP
    EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON public.%s TO app_user', tbl);
    BEGIN
      EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON public.%s TO app_admin', tbl);
    EXCEPTION WHEN OTHERS THEN NULL;
    END;
    BEGIN
      EXECUTE format('GRANT SELECT ON public.%s TO app_readonly', tbl);
    EXCEPTION WHEN OTHERS THEN NULL;
    END;
  END LOOP;
END
$grants$;

COMMIT;
