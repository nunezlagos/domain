-- migration: create_rls_sensitive
-- author: mnunez@saargo.com
-- issue: HU-25.5
-- description: RLS policies en tablas sensibles (defense-in-depth contra bugs RBAC app)
-- breaking: false
-- estimated_duration: <1s
--
-- Tablas con RLS por organization_id:
--   secrets, audit_log, otp_codes, activity_log, api_keys, sessions (cuando exista)
-- App debe ejecutar `SET LOCAL app.current_org_id = '<uuid>'` al inicio de cada tx
-- vía helper db.WithOrgTx. Si NO hay SET LOCAL, las queries devuelven 0 rows.
-- app_admin (BYPASSRLS) puede ver todo (batch jobs, migrations).
--
-- audit_log: scope organization_id IS NULL implica system event (visible para org bypass).

-- Helper: current org desde session setting; vacío si no seteado → coerce a NULL UUID.
CREATE OR REPLACE FUNCTION current_org_id() RETURNS UUID AS $$
BEGIN
  RETURN nullif(current_setting('app.current_org_id', true), '')::uuid;
EXCEPTION WHEN OTHERS THEN
  RETURN NULL;
END;
$$ LANGUAGE plpgsql STABLE;

-- NOTA: FORCE ROW LEVEL SECURITY aplica RLS incluso al OWNER de la tabla.
-- Sin FORCE, el dueño (app user de migrations) bypassea RLS y rompe el modelo.
-- BYPASSRLS role explícito (app_admin) sigue saltándose para batch jobs.

-- ===== secrets =====
ALTER TABLE secrets ENABLE ROW LEVEL SECURITY;
ALTER TABLE secrets FORCE ROW LEVEL SECURITY;
CREATE POLICY secrets_org_isolation ON secrets
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());

-- ===== audit_log =====
ALTER TABLE audit_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_log FORCE ROW LEVEL SECURITY;
CREATE POLICY audit_log_org_isolation ON audit_log
  FOR SELECT TO PUBLIC
  USING (organization_id = current_org_id() OR organization_id IS NULL);
-- INSERT no necesita filtro (app_user puede insertar cualquier; audit es write-once)
CREATE POLICY audit_log_org_insert ON audit_log
  FOR INSERT TO PUBLIC
  WITH CHECK (true);

-- ===== otp_codes =====
-- Por user, no por org (OTP es per user, pero user pertenece a org)
-- Para simplificar: usar app.current_user_id session setting.
CREATE OR REPLACE FUNCTION current_user_id() RETURNS UUID AS $$
BEGIN
  RETURN nullif(current_setting('app.current_user_id', true), '')::uuid;
EXCEPTION WHEN OTHERS THEN
  RETURN NULL;
END;
$$ LANGUAGE plpgsql STABLE;

ALTER TABLE otp_codes ENABLE ROW LEVEL SECURITY;
ALTER TABLE otp_codes FORCE ROW LEVEL SECURITY;
CREATE POLICY otp_codes_user_isolation ON otp_codes
  FOR ALL TO PUBLIC
  USING (user_id = current_user_id())
  WITH CHECK (user_id = current_user_id());

-- ===== activity_log =====
ALTER TABLE activity_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE activity_log FORCE ROW LEVEL SECURITY;
CREATE POLICY activity_log_org_isolation ON activity_log
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());

-- ===== api_keys =====
ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_keys FORCE ROW LEVEL SECURITY;
CREATE POLICY api_keys_org_isolation ON api_keys
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());

-- Re-grants explícitos: ALTER DEFAULT PRIVILEGES de migration 25 solo aplica
-- a tablas creadas por app_migrator. En tests las crea otro user, así que
-- garantizamos los grants para las tablas sensibles aquí.
GRANT SELECT, INSERT, UPDATE, DELETE ON secrets, otp_codes, activity_log, api_keys TO app_user;
GRANT SELECT, INSERT ON audit_log TO app_user;
GRANT ALL ON secrets, audit_log, otp_codes, activity_log, api_keys TO app_admin;
