-- migration: rls_tier1_observations_sessions
-- author: mnunez@saargo.com
-- issue: HU-25.5 (cierre)
-- description: RLS + FORCE en observations y sessions, Tier-1 multi-tenant
-- breaking: false
-- estimated_duration: <1s
--
-- Extiende la RLS de 000028 (secrets, audit_log, otp_codes, activity_log, api_keys)
-- a las dos tablas multi-tenant restantes que faltaban:
--   - observations: memoria de los agentes, scope por organization_id
--   - sessions: sesiones de trabajo, scope por organization_id
--
-- Ambas ya tienen organization_id NOT NULL con FK a organizations + ON DELETE CASCADE.
-- Defense-in-depth: con FORCE RLS, un bug en la app que olvide WHERE organization_id=...
-- NO causa cross-org leak; Postgres devuelve 0 rows.
--
-- Re-grants explícitos por la misma razón que 000028: ALTER DEFAULT PRIVILEGES
-- de migration 25 solo aplica a tablas creadas por app_migrator; las demás
-- las crean otros roles en tests.

-- ===== observations =====
ALTER TABLE observations ENABLE ROW LEVEL SECURITY;
ALTER TABLE observations FORCE ROW LEVEL SECURITY;
CREATE POLICY observations_org_isolation ON observations
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());

-- ===== sessions =====
ALTER TABLE sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE sessions FORCE ROW LEVEL SECURITY;
CREATE POLICY sessions_org_isolation ON sessions
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());

-- Re-grants (defense: app_user debe poder CRUD ambas)
GRANT SELECT, INSERT, UPDATE, DELETE ON observations, sessions TO app_user;
GRANT ALL ON observations, sessions TO app_admin;
