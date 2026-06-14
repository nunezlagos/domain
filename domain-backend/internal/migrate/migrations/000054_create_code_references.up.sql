CREATE TABLE code_references (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  hu_id UUID NOT NULL REFERENCES user_stories(id) ON DELETE CASCADE,
  file_path TEXT NOT NULL,
  repo VARCHAR(255) NOT NULL DEFAULT 'domain',
  branch VARCHAR(255),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(hu_id, file_path)
);

CREATE INDEX code_references_hu_id_idx ON code_references (hu_id);
CREATE INDEX code_references_file_path_idx ON code_references (file_path);

GRANT SELECT, INSERT, UPDATE, DELETE ON code_references TO app_user;
