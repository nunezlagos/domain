-- migration: add_flow_runs_hardspec
-- author: mnunez@saargo.com
-- issue: hardspec — reiteración humana opcional + auditada en la fase spec
-- description: flag hardspec por corrida. Al completar sdd-spec el orquestador
--   pausa para una reiteración humana OBLIGATORIA (el dev da OK o pide rehacer
--   una parte) y la confirmación se registra en audit_log. Default TRUE
--   (obligatorio); se desactiva pasando hardspec=false explícito. El gate reusa
--   el mecanismo de confirm (MarkStepBlocked + domain_orchestrate_confirm).
-- breaking: false
-- estimated_duration: <1s

ALTER TABLE flow_runs ADD COLUMN hardspec BOOLEAN NOT NULL DEFAULT TRUE;
