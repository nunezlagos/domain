CREATE TABLE tasks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  hu_id UUID NOT NULL REFERENCES user_stories(id) ON DELETE CASCADE,
  section VARCHAR(50) NOT NULL,
  description TEXT NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'pending',
  position INT NOT NULL DEFAULT 0,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  completed_by VARCHAR(255),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX tasks_hu_id_idx ON tasks (hu_id);
CREATE INDEX tasks_hu_section_pos_idx ON tasks (hu_id, section, position);

CREATE TABLE verification_results (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  result VARCHAR(20) NOT NULL,
  evidence TEXT,
  notes TEXT,
  verified_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  verified_by VARCHAR(255)
);

CREATE INDEX verification_task_id_idx ON verification_results (task_id);

CREATE TABLE sabotage_records (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  action TEXT NOT NULL,
  expected_failure TEXT,
  actual_result TEXT,
  restored BOOLEAN NOT NULL DEFAULT true,
  performed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sabotage_task_id_idx ON sabotage_records (task_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON tasks TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON verification_results TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON sabotage_records TO app_user;
