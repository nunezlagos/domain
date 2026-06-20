DROP INDEX IF EXISTS captured_prompts_turn_completed_idx;
ALTER TABLE captured_prompts
  DROP COLUMN IF EXISTS turn_completed_at,
  DROP COLUMN IF EXISTS estimated_tokens_out,
  DROP COLUMN IF EXISTS estimated_tokens_in,
  DROP COLUMN IF EXISTS response_chars;
