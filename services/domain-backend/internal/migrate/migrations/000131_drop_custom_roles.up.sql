-- Drop custom_roles: per-org custom roles.
-- Decisión: con single-org implícito, los 5 system roles (admin/developer/pm/viewer/owner)
-- son suficientes. custom_roles era un stub (issue-02.8) que nunca se terminó de activar.
DROP TABLE IF EXISTS custom_roles;
DROP FUNCTION IF EXISTS notify_custom_roles_changed() CASCADE;
