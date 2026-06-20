CREATE TABLE requirements (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug VARCHAR(50) NOT NULL,
  title VARCHAR(500) NOT NULL,
  description TEXT,
  status VARCHAR(20) NOT NULL DEFAULT 'active',
  priority VARCHAR(20) NOT NULL DEFAULT 'medium',
  parent_id UUID REFERENCES requirements(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX requirements_slug_idx ON requirements (slug);
CREATE INDEX requirements_status_idx ON requirements (status);
CREATE INDEX requirements_priority_idx ON requirements (priority);
CREATE INDEX requirements_parent_id_idx ON requirements (parent_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON requirements TO app_user;
