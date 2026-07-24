-- migration: 000274_file_attachments_org_rls
-- author: nunezlagos
-- issue: DOMAINSERV-112
-- description: file_attachments no tenía dimensión de tenant (sin organization_id
--   ni RLS), y las tools MCP de attachments (DOMAINSERV-79 H1) operan por
--   id/entity sin filtro de org → IDOR cross-tenant por UUID. Se agrega
--   organization_id (DEFAULT current_org_id(), autofill desde el SET LOCAL de
--   withOrgTxHandler) + RLS FORCE con aislamiento por org (mismo patrón que
--   secrets en 000028). Reads/deletes/inserts vía las tools quedan scoped a la
--   org de la sesión sin cambios en Go. Filas previas (tabla era código muerto,
--   solo usada por tests del issuebuilder) quedan con org NULL → invisibles bajo
--   RLS, lo cual es seguro.
-- breaking: no
-- duration: <1s

ALTER TABLE file_attachments ADD COLUMN IF NOT EXISTS organization_id UUID DEFAULT current_org_id();

CREATE INDEX IF NOT EXISTS file_attachments_org_idx ON file_attachments (organization_id);

ALTER TABLE file_attachments ENABLE ROW LEVEL SECURITY;
ALTER TABLE file_attachments FORCE ROW LEVEL SECURITY;
CREATE POLICY file_attachments_org_isolation ON file_attachments
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());
