









ALTER TABLE flow_runs
  ADD COLUMN exec_mode VARCHAR(20) NOT NULL DEFAULT 'auto'
    CHECK (exec_mode IN ('auto','manual','hybrid'));
