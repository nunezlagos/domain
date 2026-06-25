







































BEGIN;




ALTER TABLE IF EXISTS captured_prompts DROP CONSTRAINT IF EXISTS captured_prompts_session_id_fkey;
ALTER TABLE IF EXISTS captured_prompts DROP COLUMN     IF EXISTS session_id;
ALTER TABLE IF EXISTS verifications    DROP CONSTRAINT IF EXISTS verifications_session_id_fkey;
ALTER TABLE IF EXISTS verifications    DROP COLUMN     IF EXISTS session_id;



DROP TABLE IF EXISTS sessions                 CASCADE;
DROP TABLE IF EXISTS model_registry           CASCADE;
DROP TABLE IF EXISTS entity_state_transitions CASCADE;
DROP TABLE IF EXISTS system_state             CASCADE;
DROP TABLE IF EXISTS saga_compensation_log;
DROP TABLE IF EXISTS runtime_configs          CASCADE;
DROP TABLE IF EXISTS dead_letter_queue        CASCADE;
DROP TABLE IF EXISTS idempotency_keys         CASCADE;



DROP FUNCTION IF EXISTS entity_state_transitions_immutable() CASCADE;

COMMIT;
