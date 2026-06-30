REVOKE ALL ON skill_suggestions FROM app_readonly;
REVOKE ALL ON skill_suggestions FROM app_admin;
REVOKE ALL ON skill_suggestions FROM app_user;

DROP TABLE IF EXISTS skill_suggestions CASCADE;

DROP INDEX IF EXISTS skills_superseded_idx;
DROP INDEX IF EXISTS skills_parent_idx;

ALTER TABLE skills DROP COLUMN IF EXISTS superseded_by;
ALTER TABLE skills DROP COLUMN IF EXISTS parent_skill_id;
