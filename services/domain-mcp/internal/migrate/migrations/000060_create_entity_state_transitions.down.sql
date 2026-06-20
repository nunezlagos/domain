DROP VIEW IF EXISTS v_stuck_entities;
DROP TRIGGER IF EXISTS entity_state_transitions_no_update ON entity_state_transitions;
DROP FUNCTION IF EXISTS entity_state_transitions_immutable();
DROP TABLE IF EXISTS entity_state_transitions CASCADE;
