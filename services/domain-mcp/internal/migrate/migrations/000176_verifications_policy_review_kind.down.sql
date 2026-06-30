-- Revierte 'policy_review' del CHECK de tdd_verifications.kind.
-- Las filas existentes con kind='policy_review' violarían el CHECK
-- restaurado; el rollback asume que se purgaron antes de bajar.
ALTER TABLE tdd_verifications DROP CONSTRAINT IF EXISTS tdd_verifications_kind_check;
ALTER TABLE tdd_verifications ADD CONSTRAINT tdd_verifications_kind_check
  CHECK (kind IN ('build','test','lint','smoke','typecheck','migration','custom'));
