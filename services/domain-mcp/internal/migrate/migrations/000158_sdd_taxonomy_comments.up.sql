-- migration: 000158_sdd_taxonomy_comments
-- author: NunezLagos
-- issue: legacy
-- description: agrega COMMENT ON descriptivos a las tablas del pipeline SDD/TDD (issues, sdd_*, tdd_*, etc.)
-- breaking: no
-- estimated_duration: unknown

DO $$
DECLARE
  c RECORD;
BEGIN
  FOR c IN
    SELECT * FROM (VALUES
      ('issues',                  'Issue/HU raíz del pipeline SDD (se crea en la fase spec).'),
      ('issue_drafts',            'Borrador de issue antes de confirmarse (wizard de spec).'),
      ('issue_draft_steps_log',   'Log de pasos del wizard al armar un issue_draft.'),
      ('issue_gherkin_scenarios', 'Escenarios Gherkin (criterios de aceptación) del issue; se validan en la fase verify.'),
      ('issue_intake_payloads',   'Payload crudo de entrada (router/usuario) que origina el issue.'),
      ('issue_tasks',             'Tasks atómicas del issue, descompuestas en la fase tasks.'),
      ('issue_code_references',   'Referencias al código tocado durante la fase apply (auditoría).'),
      ('sdd_requirements',        'Requerimientos (REQ) que agrupan issues. Capa SDD (especificación).'),
      ('sdd_proposals',           'Propuesta de implementación (fase propose): scope, approach, riesgos. Capa SDD.'),
      ('sdd_designs',             'Diseño (fase design): ADRs + test_plan + sabotage_plan. Capa SDD.'),
      ('tdd_verifications',       'Verificación de un issue (fase verify): corrida de los Gherkin scenarios. Capa TDD.'),
      ('tdd_verification_results','Resultado por escenario Gherkin (fase verify). Capa TDD.'),
      ('tdd_sabotage_records',    'Mutation testing (fase judge): saboteo -> test falla -> restaura. Capa TDD.')
    ) AS t(tbl, descr)
  LOOP
    IF to_regclass('public.' || c.tbl) IS NOT NULL THEN
      EXECUTE format('COMMENT ON TABLE %I IS %L', c.tbl, c.descr);
    END IF;
  END LOOP;
END$$;
