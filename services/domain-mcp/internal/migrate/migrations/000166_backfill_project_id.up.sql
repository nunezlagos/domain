-- migration: 000166_backfill_project_id
-- author: NunezLagos
-- issue: legacy
-- description: backfill de project_id en issue_* y sdd_requirements a partir de datos existentes
-- breaking: no
-- estimated_duration: unknown

UPDATE issue_gherkin_scenarios s
SET project_id = i.project_id
FROM issues i
WHERE s.issue_id = i.id
  AND s.project_id IS NULL
  AND i.project_id IS NOT NULL;

UPDATE issue_tasks t
SET project_id = i.project_id
FROM issues i
WHERE t.issue_id = i.id
  AND t.project_id IS NULL
  AND i.project_id IS NOT NULL;

UPDATE issue_code_references c
SET project_id = i.project_id
FROM issues i
WHERE c.issue_id = i.id
  AND c.project_id IS NULL
  AND i.project_id IS NOT NULL;


UPDATE issues i
SET project_id = r.project_id
FROM sdd_requirements r
WHERE i.req_id = r.id
  AND i.project_id IS NULL
  AND r.project_id IS NOT NULL;



UPDATE issue_gherkin_scenarios s
SET project_id = i.project_id
FROM issues i
WHERE s.issue_id = i.id
  AND s.project_id IS NULL
  AND i.project_id IS NOT NULL;

UPDATE issue_tasks t
SET project_id = i.project_id
FROM issues i
WHERE t.issue_id = i.id
  AND t.project_id IS NULL
  AND i.project_id IS NOT NULL;

UPDATE issue_code_references c
SET project_id = i.project_id
FROM issues i
WHERE c.issue_id = i.id
  AND c.project_id IS NULL
  AND i.project_id IS NOT NULL;






UPDATE sdd_requirements r
SET project_id = i.project_id
FROM issues i
WHERE i.req_id = r.id
  AND r.project_id IS NULL
  AND i.project_id IS NOT NULL;



DELETE FROM issue_gherkin_scenarios WHERE project_id IS NULL;
DELETE FROM issue_tasks            WHERE project_id IS NULL;
DELETE FROM issue_code_references  WHERE project_id IS NULL;
DELETE FROM issue_drafts           WHERE project_id IS NULL;
DELETE FROM issue_intake_payloads  WHERE project_id IS NULL;





DELETE FROM issues                 WHERE project_id IS NULL;
DELETE FROM sdd_requirements       WHERE project_id IS NULL;
