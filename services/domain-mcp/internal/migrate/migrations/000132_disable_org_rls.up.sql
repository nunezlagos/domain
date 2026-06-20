-- issue-21.6 Fase A: deshabilitar RLS basada en organización (single-org).
--
-- Con single-org, el aislamiento por current_org_id() deja de tener sentido:
-- todo el dataset pertenece a la única org. Esta migración DESHABILITA RLS en
-- las tablas org-scoped (no dropea las policies — quedan definidas pero inertes,
-- el down solo re-habilita → reversible). El drop definitivo de policies +
-- función current_org_id() va en Fase C (junto con DROP COLUMN organization_id).
--
-- NO se toca otp_codes: su RLS es user-isolation (current_user_id()), no org.
--
-- Nota: app_user (NOBYPASSRLS) pasa a ver todas las filas de estas tablas; en
-- single-org eso es exactamente el conjunto de la única org (pérdida de defense
-- in depth aceptada — ver design.md de issue-21.6).

ALTER TABLE activity_log                  DISABLE ROW LEVEL SECURITY;
ALTER TABLE api_keys                      DISABLE ROW LEVEL SECURITY;
ALTER TABLE audit_log                     DISABLE ROW LEVEL SECURITY;
ALTER TABLE captured_prompts              DISABLE ROW LEVEL SECURITY;
ALTER TABLE clients                       DISABLE ROW LEVEL SECURITY;
ALTER TABLE observations                  DISABLE ROW LEVEL SECURITY;
ALTER TABLE organizations                 DISABLE ROW LEVEL SECURITY;
ALTER TABLE project_index_runs            DISABLE ROW LEVEL SECURITY;
ALTER TABLE project_policies              DISABLE ROW LEVEL SECURITY;
ALTER TABLE project_policy_versions       DISABLE ROW LEVEL SECURITY;
ALTER TABLE project_repositories          DISABLE ROW LEVEL SECURITY;
ALTER TABLE projects                      DISABLE ROW LEVEL SECURITY;
ALTER TABLE project_ticket_comments       DISABLE ROW LEVEL SECURITY;
ALTER TABLE project_tickets               DISABLE ROW LEVEL SECURITY;
ALTER TABLE project_ticket_status_history DISABLE ROW LEVEL SECURITY;
ALTER TABLE secrets                       DISABLE ROW LEVEL SECURITY;
ALTER TABLE sessions                      DISABLE ROW LEVEL SECURITY;
ALTER TABLE users                         DISABLE ROW LEVEL SECURITY;
ALTER TABLE verifications                 DISABLE ROW LEVEL SECURITY;
