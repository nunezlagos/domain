REVOKE ALL ON skill_metrics_weekly FROM app_readonly;
REVOKE ALL ON skill_metrics_weekly FROM app_admin;
REVOKE ALL ON skill_metrics_weekly FROM app_user;
REVOKE ALL ON skill_metrics_daily FROM app_readonly;
REVOKE ALL ON skill_metrics_daily FROM app_admin;
REVOKE ALL ON skill_metrics_daily FROM app_user;

DROP TABLE IF EXISTS skill_metrics_weekly CASCADE;
DROP TABLE IF EXISTS skill_metrics_daily CASCADE;
