









ALTER TABLE captured_prompts
  ADD COLUMN IF NOT EXISTS response_chars        INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS estimated_tokens_in   INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS estimated_tokens_out  INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS turn_completed_at     TIMESTAMPTZ;


UPDATE captured_prompts
   SET estimated_tokens_in = CEIL(char_count / 4.0)::INT
   WHERE estimated_tokens_in = 0 AND char_count > 0;

CREATE INDEX IF NOT EXISTS captured_prompts_turn_completed_idx
  ON captured_prompts(organization_id, turn_completed_at DESC)
  WHERE turn_completed_at IS NOT NULL;
