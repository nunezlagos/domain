

























BEGIN;

ALTER TABLE org_enrollment_tokens RENAME TO enrollment_tokens;

ALTER INDEX org_enrollment_tokens_pkey                  RENAME TO enrollment_tokens_pkey;
ALTER INDEX org_enrollment_tokens_prefix_idx            RENAME TO enrollment_tokens_prefix_idx;
ALTER INDEX org_enrollment_tokens_singleton_active_uniq RENAME TO enrollment_tokens_singleton_active_uniq;
ALTER INDEX org_enrollment_tokens_status_idx            RENAME TO enrollment_tokens_status_idx;

ALTER TABLE enrollment_tokens
  RENAME CONSTRAINT org_enrollment_tokens_created_by_user_id_fkey
  TO enrollment_tokens_created_by_user_id_fkey;
ALTER TABLE enrollment_tokens
  RENAME CONSTRAINT org_enrollment_tokens_role_check
  TO enrollment_tokens_role_check;

COMMIT;
