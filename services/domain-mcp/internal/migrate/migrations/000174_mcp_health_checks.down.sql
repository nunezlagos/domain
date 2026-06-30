REVOKE ALL ON mcp_health_checks FROM app_user;
REVOKE ALL ON mcp_health_checks FROM app_admin;

DROP TABLE IF EXISTS mcp_health_checks CASCADE;
