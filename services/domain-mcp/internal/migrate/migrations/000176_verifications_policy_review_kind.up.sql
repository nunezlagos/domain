-- sdd-review: agrega 'policy_review' al CHECK de tdd_verifications.kind.
-- La fase sdd-review (revisor de implementación, entre judge y archive)
-- abre un checkpoint con kind='policy_review' (un item por policy/skill
-- evaluada) reusando los tools domain_verify_*. No requiere tabla nueva.
ALTER TABLE tdd_verifications DROP CONSTRAINT IF EXISTS tdd_verifications_kind_check;
ALTER TABLE tdd_verifications ADD CONSTRAINT tdd_verifications_kind_check
  CHECK (kind IN ('build','test','lint','smoke','typecheck','migration','policy_review','custom'));
