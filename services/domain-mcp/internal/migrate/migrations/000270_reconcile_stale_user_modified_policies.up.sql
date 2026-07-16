-- migration: 000270_reconcile_stale_user_modified_policies
-- author: nunezlagos
-- issue: DOMAINSERV-34
-- description: resetea is_user_modified en agent-protocol/agent-voice. Su flag
--   quedó en true por una edición manual vieja (voseo + typo pre-neutralización),
--   no una customización a preservar. Con el flag en false, el PlatformPoliciesSeeder
--   (Version 18) reaplica el body neutral del fuente. NO toca sdd-auto-trigger, cuya
--   edición (formato markdown) sí es legítima.
-- breaking: no
-- duration: <1s
UPDATE platform_policies
SET is_user_modified = false
WHERE slug IN ('agent-protocol', 'agent-voice')
  AND is_user_modified;
