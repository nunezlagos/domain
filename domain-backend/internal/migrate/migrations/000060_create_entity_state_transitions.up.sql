-- migration: create_entity_state_transitions
-- author: nunezlagos
-- issue: HU-04.10
-- description: audit immutable de transiciones de estado cross-entity
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE entity_state_transitions (
  id BIGSERIAL PRIMARY KEY,
  entity_kind VARCHAR(30) NOT NULL
    CHECK (entity_kind IN ('intake','req','hu','sync_state','proposal','design','task')),
  entity_id UUID NOT NULL,
  from_state VARCHAR(40),                  -- NULL en creación
  to_state VARCHAR(40) NOT NULL,
  actor_kind VARCHAR(20) NOT NULL
    CHECK (actor_kind IN ('user','agent','system','external')),
  actor_id UUID,
  actor_name VARCHAR(120),
  reason TEXT,
  context JSONB,
  tx_id UUID,                              -- correlación con request_id / job_id
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX entity_state_transitions_entity_idx
  ON entity_state_transitions (entity_kind, entity_id, occurred_at);
CREATE INDEX entity_state_transitions_to_state_idx
  ON entity_state_transitions (entity_kind, to_state, occurred_at);
CREATE INDEX entity_state_transitions_actor_idx
  ON entity_state_transitions (actor_id) WHERE actor_id IS NOT NULL;

-- Trigger anti-UPDATE/DELETE: append-only.
CREATE OR REPLACE FUNCTION entity_state_transitions_immutable() RETURNS TRIGGER AS $$
BEGIN
  RAISE EXCEPTION 'entity_state_transitions is append-only (op=%)', TG_OP;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER entity_state_transitions_no_update
  BEFORE UPDATE OR DELETE ON entity_state_transitions
  FOR EACH ROW EXECUTE FUNCTION entity_state_transitions_immutable();

-- Stuck detector materializa entidades sin transición reciente.
CREATE OR REPLACE VIEW v_stuck_entities AS
SELECT
  entity_kind,
  entity_id,
  to_state AS current_state,
  occurred_at AS last_transition_at,
  EXTRACT(EPOCH FROM (now() - occurred_at)) / 3600 AS hours_in_state
FROM (
  SELECT DISTINCT ON (entity_kind, entity_id)
    entity_kind, entity_id, to_state, occurred_at
  FROM entity_state_transitions
  ORDER BY entity_kind, entity_id, occurred_at DESC
) latest
WHERE to_state NOT IN ('done','archived','rejected','committed','expired','abandoned');

GRANT SELECT, INSERT ON entity_state_transitions TO app_user;
GRANT USAGE, SELECT ON SEQUENCE entity_state_transitions_id_seq TO app_user;
GRANT SELECT ON v_stuck_entities TO app_user;
