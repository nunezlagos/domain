CREATE TABLE IF NOT EXISTS known_errors (
  fingerprint bytea PRIMARY KEY,
  name text NOT NULL,
  description text NULL,
  remediation text NULL,
  recoverable boolean NOT NULL DEFAULT true,
  auto_heal_action text NOT NULL DEFAULT 'none'
    CHECK (auto_heal_action IN ('retry','clear_cache','restart_worker','none')),
  action_params jsonb NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
