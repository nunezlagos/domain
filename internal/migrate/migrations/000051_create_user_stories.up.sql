CREATE TABLE user_stories (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug VARCHAR(50) UNIQUE NOT NULL,
  title VARCHAR(500) NOT NULL,
  description TEXT,
  status VARCHAR(20) NOT NULL DEFAULT 'proposed',
  priority VARCHAR(20) NOT NULL DEFAULT 'medium',
  req_id UUID NOT NULL REFERENCES requirements(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX user_stories_req_id_idx ON user_stories (req_id);
CREATE INDEX user_stories_status_idx ON user_stories (status);

CREATE TABLE gherkin_scenarios (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  hu_id UUID NOT NULL REFERENCES user_stories(id) ON DELETE CASCADE,
  feature VARCHAR(255) NOT NULL,
  scenario VARCHAR(500) NOT NULL,
  given TEXT[] NOT NULL,
  when_text TEXT NOT NULL,
  then_rows TEXT[] NOT NULL,
  position INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX gherkin_hu_id_idx ON gherkin_scenarios (hu_id);
