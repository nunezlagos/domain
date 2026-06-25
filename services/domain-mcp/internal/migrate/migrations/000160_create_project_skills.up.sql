









CREATE TABLE project_skills (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  skill_id    UUID NOT NULL REFERENCES skills(id)   ON DELETE CASCADE,
  is_enabled  BOOLEAN NOT NULL DEFAULT TRUE,
  created_by  UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (project_id, skill_id)
);

CREATE INDEX project_skills_project_idx ON project_skills(project_id);
CREATE INDEX project_skills_skill_idx   ON project_skills(skill_id);
