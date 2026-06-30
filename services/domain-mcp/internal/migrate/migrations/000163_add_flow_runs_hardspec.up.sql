-- migration: 000163_add_flow_runs_hardspec
-- author: NunezLagos
-- issue: legacy
-- description: columna hardspec (boolean, default true) en flow_runs
-- breaking: no
-- estimated_duration: unknown

ALTER TABLE flow_runs ADD COLUMN hardspec BOOLEAN NOT NULL DEFAULT TRUE;
