-- migration: backfill_project_id
-- author: mnunez@saargo.com
-- issue: scoping por proyecto Fase 2 — preparar el SET NOT NULL (000167)
-- description: rellena project_id en las tablas core antes de imponer NOT NULL.
--   project_id se agrego nullable en 000161; el wizard/orquestador ya lo pueblan
--   en los caminos principales, pero quedan filas con project_id NULL que
--   romperian un SET NOT NULL. Esta migracion las resuelve en dos pasos:
--
--   1) BACKFILL por derivacion: las tablas satelite heredan project_id de su
--      padre via JOIN (issue_id -> issues, req_id -> sdd_requirements). Donde el
--      padre ya tiene project_id, la fila hija lo copia.
--
--   2) BORRADO de huerfanas: las filas que siguen NULL tras el backfill no son
--      derivables (tipicamente artefactos de test en greenfield: flow_runs sin
--      proyecto, issue_drafts/intake_payloads de smoke tests, sdd_requirements
--      raiz sin scope). En un entorno greenfield NO hay datos reales sin
--      proyecto, asi que se BORRAN: dejarlas bloquearia el NOT NULL y mantenerlas
--      con un project_id inventado corromperia el scoping. El borrado respeta el
--      orden de dependencias (satelites e issues caen por CASCADE/SET NULL de sus
--      FKs, pero borramos explicitamente para ser deterministas).
-- breaking: false (greenfield: solo toca filas sin proyecto, que son de test)
-- estimated_duration: <1s (tablas casi vacias, UPDATEs por JOIN indexado)

-- 1) BACKFILL: satelites heredan de issues via issue_id.
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

-- issues hereda de sdd_requirements via req_id (donde issues.project_id sigue NULL).
UPDATE issues i
SET project_id = r.project_id
FROM sdd_requirements r
WHERE i.req_id = r.id
  AND i.project_id IS NULL
  AND r.project_id IS NOT NULL;

-- Re-pasada de satelites de issues: por si el issue padre recien obtuvo
-- project_id en el UPDATE anterior (cadena req -> issue -> satelite).
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

-- 1b) Reverse-backfill defensivo: si un REQ quedo con project_id NULL pero tiene
--     issues con project_id valido (escritas por la app antes del guard), heredamos
--     el project_id de ese issue. Evita violar el RESTRICT de issues.req_id cuando
--     mas abajo se borren los REQ NULL (un issue sobreviviente no puede apuntar a un
--     REQ borrado). Idempotente.
UPDATE sdd_requirements r
SET project_id = i.project_id
FROM issues i
WHERE i.req_id = r.id
  AND r.project_id IS NULL
  AND i.project_id IS NOT NULL;

-- 2) BORRADO de huerfanas no derivables (greenfield: artefactos de test).
--    Orden hoja -> raiz para no depender solo del CASCADE/SET NULL de las FKs.
DELETE FROM issue_gherkin_scenarios WHERE project_id IS NULL;
DELETE FROM issue_tasks            WHERE project_id IS NULL;
DELETE FROM issue_code_references  WHERE project_id IS NULL;
DELETE FROM issue_drafts           WHERE project_id IS NULL;
DELETE FROM issue_intake_payloads  WHERE project_id IS NULL;
-- flow_runs NO se borra: es dual-use (orquestador SDD lo scopea, pero el runner
-- generico de flows corre sin proyecto y deja project_id NULL legitimamente).
-- Por eso flow_runs queda EXCLUIDA del NOT NULL (ver 000167).
-- Para issues borramos solo las que NO tienen REQ con project_id (las huerfanas
-- de test); las issues con REQ valido ya fueron backfilleadas arriba.
DELETE FROM issues                 WHERE project_id IS NULL;
DELETE FROM sdd_requirements       WHERE project_id IS NULL;
