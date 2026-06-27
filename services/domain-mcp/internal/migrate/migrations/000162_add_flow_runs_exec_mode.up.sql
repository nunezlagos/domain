-- migration: 000162_add_flow_runs_exec_mode
-- author: NunezLagos
-- issue: legacy
-- description: columna exec_mode en flow_runs (auto/manual/hybrid)
-- breaking: no
-- estimated_duration: unknown

ALTER TABLE flow_runs
  ADD COLUMN exec_mode VARCHAR(20) NOT NULL DEFAULT 'auto'
    CHECK (exec_mode IN ('auto','manual','hybrid'));
