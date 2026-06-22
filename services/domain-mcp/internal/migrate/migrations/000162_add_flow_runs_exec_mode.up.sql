-- migration: add_flow_runs_exec_mode
-- author: mnunez@saargo.com
-- issue: modo de ejecución SDD (auto / manual / hibrido)
-- description: exec_mode por corrida. auto = corre sin pausar; manual = pausa
--   y pide aprobación tras CADA fase; hibrido = pausa solo en fases clave
--   (spec/design/apply/judge). El gate reusa el mecanismo de confirm existente
--   (MarkStepBlocked + domain_orchestrate_confirm). Greenfield: instantáneo.
-- breaking: false
-- estimated_duration: <1s

ALTER TABLE flow_runs
  ADD COLUMN exec_mode VARCHAR(20) NOT NULL DEFAULT 'auto'
    CHECK (exec_mode IN ('auto','manual','hybrid'));
