DROP TRIGGER IF EXISTS custom_roles_notify_mod ON custom_roles;
DROP FUNCTION IF EXISTS notify_custom_roles_changed();
DROP TABLE IF EXISTS custom_roles CASCADE;
