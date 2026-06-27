REVOKE ALL ON skill_ab_test_results FROM app_readonly;
REVOKE ALL ON skill_ab_test_results FROM app_admin;
REVOKE ALL ON skill_ab_test_results FROM app_user;

REVOKE ALL ON skill_ab_tests FROM app_readonly;
REVOKE ALL ON skill_ab_tests FROM app_admin;
REVOKE ALL ON skill_ab_tests FROM app_user;

DROP TABLE IF EXISTS skill_ab_test_results CASCADE;

DROP INDEX IF EXISTS skill_ab_tests_status_idx;
DROP INDEX IF EXISTS skill_ab_tests_slug_idx;
DROP INDEX IF EXISTS skill_ab_tests_running_uniq;

DROP TABLE IF EXISTS skill_ab_tests CASCADE;
