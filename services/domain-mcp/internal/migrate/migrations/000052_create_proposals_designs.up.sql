CREATE TABLE proposals (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  hu_id UUID NOT NULL REFERENCES user_stories(id) ON DELETE CASCADE,
  version INT NOT NULL DEFAULT 1,
  status VARCHAR(20) NOT NULL DEFAULT 'draft',
  intention TEXT NOT NULL,
  scope TEXT NOT NULL,
  approach TEXT NOT NULL,
  risks TEXT,
  testing_notes TEXT,
  rejection_reason TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(hu_id, version)
);

CREATE INDEX proposals_status_idx ON proposals (status);

CREATE TABLE designs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  hu_id UUID NOT NULL REFERENCES user_stories(id) ON DELETE CASCADE,
  proposal_id UUID REFERENCES proposals(id) ON DELETE SET NULL,
  version INT NOT NULL DEFAULT 1,
  status VARCHAR(20) NOT NULL DEFAULT 'draft',
  arch_decisions TEXT NOT NULL,
  alternatives TEXT,
  data_flow TEXT,
  tdd_plan TEXT,
  risks_mitigation TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(hu_id, version)
);

GRANT SELECT, INSERT, UPDATE, DELETE ON proposals TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON designs TO app_user;
