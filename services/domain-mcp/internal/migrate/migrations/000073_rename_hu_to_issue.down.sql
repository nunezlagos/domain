




BEGIN;


UPDATE entity_state_transitions SET entity_kind = 'hu' WHERE entity_kind = 'issue';


ALTER INDEX IF EXISTS issues_pkey RENAME TO user_stories_pkey;
ALTER INDEX IF EXISTS issues_organization_id_slug_unique RENAME TO user_stories_organization_id_slug_unique;
ALTER INDEX IF EXISTS issues_organization_id_idx RENAME TO user_stories_organization_id_idx;
ALTER INDEX IF EXISTS issues_status_idx RENAME TO user_stories_status_idx;
ALTER INDEX IF EXISTS issues_req_id_idx RENAME TO user_stories_req_id_idx;

ALTER INDEX IF EXISTS issue_drafts_pkey RENAME TO hu_drafts_pkey;
ALTER INDEX IF EXISTS issue_drafts_created_by_idx RENAME TO hu_drafts_created_by_idx;
ALTER INDEX IF EXISTS issue_drafts_status_idx RENAME TO hu_drafts_status_idx;

ALTER INDEX IF EXISTS issue_draft_steps_log_pkey RENAME TO hu_draft_steps_log_pkey;
ALTER INDEX IF EXISTS issue_draft_steps_log_issue_draft_id_idx RENAME TO hu_draft_steps_log_draft_id_idx;


DROP TRIGGER IF EXISTS set_updated_at_issues ON issues;
DROP TRIGGER IF EXISTS set_updated_at_issue_drafts ON issue_drafts;


ALTER TABLE intake_payloads RENAME COLUMN committed_issue_id TO committed_hu_id;
ALTER TABLE code_references RENAME COLUMN issue_id TO hu_id;
ALTER TABLE tasks RENAME COLUMN issue_id TO hu_id;
ALTER TABLE designs RENAME COLUMN issue_id TO hu_id;
ALTER TABLE proposals RENAME COLUMN issue_id TO hu_id;
ALTER TABLE gherkin_scenarios RENAME COLUMN issue_id TO hu_id;

ALTER TABLE issue_draft_steps_log RENAME COLUMN issue_draft_id TO draft_id;


ALTER TABLE issue_draft_steps_log RENAME TO hu_draft_steps_log;
ALTER TABLE issue_drafts RENAME TO hu_drafts;
ALTER TABLE issues RENAME TO user_stories;


CREATE TRIGGER set_updated_at_user_stories
  BEFORE UPDATE ON user_stories
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER set_updated_at_hu_drafts
  BEFORE UPDATE ON hu_drafts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;
