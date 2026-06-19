-- migration: rename_sdd_tdd_issue_taxonomy
-- author: mnunez@saargo.com
-- issue: REQ-42.5 (schema-naming-taxonomy — dominio SDD/TDD + capa issue)
-- description: aplica la taxonomía de prefijos a las tablas del pipeline
--   SDD/TDD y de la capa issue, en una sola transacción atómica (patrón
--   000146 + 000073). Renombra tabla + índices + constraints (incluido
--   re-prefijado y alineación de nombres legacy `hu_id_*` → `issue_id_*`
--   que sobrevivieron al rename de columnas de 000073).
--
--   Tablas (11) y su grupo:
--     SDD   : requirements → sdd_requirements
--             proposals    → sdd_proposals
--             designs      → sdd_designs
--     ISSUE : gherkin_scenarios → issue_gherkin_scenarios
--             tasks             → issue_tasks
--             code_references   → issue_code_references
--             intake_payloads   → issue_intake_payloads
--     TDD   : verifications        → tdd_verifications
--             verification_results → tdd_verification_results
--             sabotage_records     → tdd_sabotage_records (mutation/sabotage
--               testing; preservada como capa TDD por decisión del usuario,
--               NO se dropea. FK saliente task_id → issue_tasks intacta por OID)
--
--   Notas de coordinación (TODO en la MISMA tx para que las FKs por OID
--   queden consistentes):
--     - designs.proposal_id → proposals  y  verification_results.task_id → tasks
--       referencian tablas renombradas en este mismo lote. Postgres mantiene
--       la FK por OID; sólo se re-prefija el nombre del constraint.
--     - FKs entrantes que sobreviven solas (no se tocan): issues.req_id,
--       intake_payloads.committed_req_id (sobre la propia tabla), etc.
--
--   Triggers:
--     - trg_set_updated_at es GENÉRICO (sin sufijo de tabla) → sobrevive al
--       RENAME automáticamente, NO se toca.
--     - intake_payloads tiene además set_updated_at_intake_payloads (sufijo
--       legacy): no se rompe con el RENAME (vive en la tabla), pero por
--       consistencia se hace DROP + CREATE con el nuevo nombre (patrón 000073).
--
--   RLS: ninguna de estas tablas tiene policies vigentes (sólo audit_log y
--     otp_codes las tienen). verifications tuvo RLS+organization_id (000111)
--     pero 000132 dropeó la columna y deshabilitó RLS; queda el flag residual
--     FORCE inerte → se limpia con NO FORCE (opcional, no rompe nada).
--
--   Sin sequences: todas estas tablas usan UUID PK con gen_random_uuid().
--   down: invierte todos los RENAME (atómico).
-- breaking: true (cambia nombres de tablas usados en SQL embebido del backend;
--   requiere desplegar el código actualizado en el MISMO deploy — ver tasks.md)
-- estimated_duration: <2s (renames = metadata-only en Postgres)

BEGIN;

-- =====================================================================
-- 1) SDD: requirements → sdd_requirements
-- =====================================================================
ALTER TABLE IF EXISTS requirements RENAME TO sdd_requirements;

ALTER INDEX IF EXISTS requirements_pkey            RENAME TO sdd_requirements_pkey;
ALTER INDEX IF EXISTS requirements_parent_id_idx   RENAME TO sdd_requirements_parent_id_idx;
ALTER INDEX IF EXISTS requirements_priority_idx    RENAME TO sdd_requirements_priority_idx;
ALTER INDEX IF EXISTS requirements_slug_idx        RENAME TO sdd_requirements_slug_idx;
ALTER INDEX IF EXISTS requirements_status_idx      RENAME TO sdd_requirements_status_idx;

ALTER TABLE sdd_requirements RENAME CONSTRAINT requirements_parent_id_fkey TO sdd_requirements_parent_id_fkey;

-- =====================================================================
-- 2) SDD: proposals → sdd_proposals
--    (objetos conservan nombres LEGACY era HU: *_hu_id_* → *_issue_id_*)
-- =====================================================================
ALTER TABLE IF EXISTS proposals RENAME TO sdd_proposals;

ALTER INDEX IF EXISTS proposals_pkey                 RENAME TO sdd_proposals_pkey;
ALTER INDEX IF EXISTS proposals_hu_id_version_key    RENAME TO sdd_proposals_issue_id_version_key;
ALTER INDEX IF EXISTS proposals_status_idx           RENAME TO sdd_proposals_status_idx;

-- NOTA: el UNIQUE(hu_id,version) es index-backed; ALTER INDEX (arriba) ya renombró
-- también el constraint. Un RENAME CONSTRAINT adicional fallaría (nombre ya inexistente).
ALTER TABLE sdd_proposals RENAME CONSTRAINT proposals_hu_id_fkey        TO sdd_proposals_issue_id_fkey;

-- =====================================================================
-- 3) SDD: designs → sdd_designs
--    (FK proposal_id → sdd_proposals queda consistente por OID; sólo
--     se re-prefija el nombre del constraint)
-- =====================================================================
ALTER TABLE IF EXISTS designs RENAME TO sdd_designs;

ALTER INDEX IF EXISTS designs_pkey               RENAME TO sdd_designs_pkey;
ALTER INDEX IF EXISTS designs_hu_id_version_key  RENAME TO sdd_designs_issue_id_version_key;
ALTER INDEX IF EXISTS designs_status_idx         RENAME TO sdd_designs_status_idx;

-- NOTA: designs_hu_id_version_key es index-backed (ya renombrado por ALTER INDEX).
ALTER TABLE sdd_designs RENAME CONSTRAINT designs_hu_id_fkey         TO sdd_designs_issue_id_fkey;
ALTER TABLE sdd_designs RENAME CONSTRAINT designs_proposal_id_fkey   TO sdd_designs_proposal_id_fkey;

-- =====================================================================
-- 4) ISSUE: gherkin_scenarios → issue_gherkin_scenarios
--    MOVIDO a 000152 (HU 42.6, migration dedicada). NO se renombra acá para
--    evitar doble-rename: 000152 corre después y fallaría con "relation
--    gherkin_scenarios does not exist".
-- =====================================================================

-- =====================================================================
-- 5) ISSUE: tasks → issue_tasks
--    (FKs entrantes verification_results.task_id y sabotage_records.task_id
--     se mantienen por OID; verification_results se renombra abajo)
-- =====================================================================
ALTER TABLE IF EXISTS tasks RENAME TO issue_tasks;

ALTER INDEX IF EXISTS tasks_pkey RENAME TO issue_tasks_pkey;

ALTER TABLE issue_tasks RENAME CONSTRAINT tasks_hu_id_fkey TO issue_tasks_issue_id_fkey;

-- =====================================================================
-- 6) ISSUE: code_references → issue_code_references
-- =====================================================================
ALTER TABLE IF EXISTS code_references RENAME TO issue_code_references;

ALTER INDEX IF EXISTS code_references_pkey                 RENAME TO issue_code_references_pkey;
ALTER INDEX IF EXISTS code_references_hu_id_idx            RENAME TO issue_code_references_issue_id_idx;
ALTER INDEX IF EXISTS code_references_hu_id_file_path_key  RENAME TO issue_code_references_issue_id_file_path_key;
ALTER INDEX IF EXISTS code_references_file_path_idx        RENAME TO issue_code_references_file_path_idx;
ALTER INDEX IF EXISTS code_references_status_idx           RENAME TO issue_code_references_status_idx;

-- NOTA: code_references_hu_id_file_path_key es index-backed (ya renombrado por ALTER INDEX).
ALTER TABLE issue_code_references RENAME CONSTRAINT code_references_hu_id_fkey          TO issue_code_references_issue_id_fkey;

-- =====================================================================
-- 7) ISSUE: intake_payloads → issue_intake_payloads
--    (trigger legacy con sufijo de tabla → DROP + CREATE; el genérico queda)
-- =====================================================================
ALTER TABLE IF EXISTS intake_payloads RENAME TO issue_intake_payloads;

ALTER INDEX IF EXISTS intake_payloads_pkey          RENAME TO issue_intake_payloads_pkey;
ALTER INDEX IF EXISTS intake_payloads_reviewer_idx  RENAME TO issue_intake_payloads_reviewer_idx;
ALTER INDEX IF EXISTS intake_payloads_source_idx    RENAME TO issue_intake_payloads_source_idx;
ALTER INDEX IF EXISTS intake_payloads_status_idx    RENAME TO issue_intake_payloads_status_idx;

ALTER TABLE issue_intake_payloads RENAME CONSTRAINT intake_payloads_source_check             TO issue_intake_payloads_source_check;
ALTER TABLE issue_intake_payloads RENAME CONSTRAINT intake_payloads_status_check             TO issue_intake_payloads_status_check;
ALTER TABLE issue_intake_payloads RENAME CONSTRAINT intake_payloads_committed_hu_id_fkey     TO issue_intake_payloads_committed_issue_id_fkey;
ALTER TABLE issue_intake_payloads RENAME CONSTRAINT intake_payloads_committed_req_id_fkey    TO issue_intake_payloads_committed_req_id_fkey;
ALTER TABLE issue_intake_payloads RENAME CONSTRAINT intake_payloads_reviewer_id_fkey         TO issue_intake_payloads_reviewer_id_fkey;

DROP TRIGGER IF EXISTS set_updated_at_intake_payloads ON issue_intake_payloads;
CREATE TRIGGER set_updated_at_issue_intake_payloads
  BEFORE UPDATE ON issue_intake_payloads
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- =====================================================================
-- 8) TDD: verifications → tdd_verifications
--    (índices legacy conservan 'org' en el nombre aunque la columna
--     organization_id ya fue dropeada en 000132 → se quita 'org')
-- =====================================================================
ALTER TABLE IF EXISTS verifications RENAME TO tdd_verifications;

ALTER INDEX IF EXISTS verifications_pkey                    RENAME TO tdd_verifications_pkey;
ALTER INDEX IF EXISTS verifications_org_project_status_idx  RENAME TO tdd_verifications_project_status_idx;
ALTER INDEX IF EXISTS verifications_org_session_idx         RENAME TO tdd_verifications_session_idx;
ALTER INDEX IF EXISTS verifications_pending_idx             RENAME TO tdd_verifications_pending_idx;

ALTER TABLE tdd_verifications RENAME CONSTRAINT verifications_kind_check        TO tdd_verifications_kind_check;
ALTER TABLE tdd_verifications RENAME CONSTRAINT verifications_status_check      TO tdd_verifications_status_check;
ALTER TABLE tdd_verifications RENAME CONSTRAINT verifications_project_id_fkey   TO tdd_verifications_project_id_fkey;
-- NOTA: session_id (FK + columna) fue dropeada en 000149 (sessions legacy) — no hay constraint que renombrar.
ALTER TABLE tdd_verifications RENAME CONSTRAINT verifications_user_id_fkey      TO tdd_verifications_user_id_fkey;

-- limpiar flag FORCE residual (sin policy es inerte; cosmético)
ALTER TABLE tdd_verifications NO FORCE ROW LEVEL SECURITY;

-- =====================================================================
-- 9) TDD: verification_results → tdd_verification_results
--    (FK task_id → issue_tasks consistente por OID; sólo re-prefijo)
-- =====================================================================
ALTER TABLE IF EXISTS verification_results RENAME TO tdd_verification_results;

ALTER INDEX IF EXISTS verification_results_pkey RENAME TO tdd_verification_results_pkey;

ALTER TABLE tdd_verification_results RENAME CONSTRAINT verification_results_task_id_fkey TO tdd_verification_results_task_id_fkey;

-- =====================================================================
-- 10) TDD: sabotage_records → tdd_sabotage_records
--     (mutation/sabotage testing; FK task_id → issue_tasks consistente por
--      OID — issue_tasks se renombra arriba en este mismo lote. SIN sequence:
--      id no-serial. Solo re-prefijo de tabla + índices + constraint FK)
-- =====================================================================
ALTER TABLE IF EXISTS sabotage_records RENAME TO tdd_sabotage_records;

ALTER INDEX IF EXISTS sabotage_records_pkey       RENAME TO tdd_sabotage_records_pkey;
ALTER INDEX IF EXISTS sabotage_records_status_idx RENAME TO tdd_sabotage_records_status_idx;
ALTER INDEX IF EXISTS sabotage_task_id_idx        RENAME TO tdd_sabotage_records_task_id_idx;

ALTER TABLE tdd_sabotage_records RENAME CONSTRAINT sabotage_records_task_id_fkey TO tdd_sabotage_records_task_id_fkey;

-- =====================================================================
-- 11) GRANT defensivo (por si app_user/app_admin no auto-heredan)
-- =====================================================================
DO $grants$
DECLARE
  tbl text;
BEGIN
  FOR tbl IN SELECT unnest(ARRAY[
    'sdd_requirements', 'sdd_proposals', 'sdd_designs',
    'issue_tasks', 'issue_code_references', 'issue_intake_payloads',
    'tdd_verifications', 'tdd_verification_results', 'tdd_sabotage_records'
  ]) LOOP
    BEGIN
      EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON public.%s TO app_user', tbl);
    EXCEPTION WHEN OTHERS THEN NULL;
    END;
    BEGIN
      EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON public.%s TO app_admin', tbl);
    EXCEPTION WHEN OTHERS THEN NULL;
    END;
    BEGIN
      EXECUTE format('GRANT SELECT ON public.%s TO app_readonly', tbl);
    EXCEPTION WHEN OTHERS THEN NULL;
    END;
  END LOOP;
END
$grants$;

COMMIT;
