REVOKE ALL ON skill_feedback_daily FROM app_user;
REVOKE ALL ON skill_feedback_daily FROM app_admin;
REVOKE ALL ON skill_feedback FROM app_user;
REVOKE ALL ON skill_feedback FROM app_admin;

DROP TABLE IF EXISTS skill_feedback_daily CASCADE;
DROP TABLE IF EXISTS skill_feedback CASCADE;
