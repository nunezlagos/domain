









BEGIN;

ALTER TABLE enrollment_tokens RENAME TO org_enrollment_tokens;

ALTER INDEX enrollment_tokens_pkey                  RENAME TO org_enrollment_tokens_pkey;
ALTER INDEX enrollment_tokens_prefix_idx            RENAME TO org_enrollment_tokens_prefix_idx;
ALTER INDEX enrollment_tokens_singleton_active_uniq RENAME TO org_enrollment_tokens_singleton_active_uniq;
ALTER INDEX enrollment_tokens_status_idx            RENAME TO org_enrollment_tokens_status_idx;

ALTER TABLE org_enrollment_tokens
  RENAME CONSTRAINT enrollment_tokens_created_by_user_id_fkey
  TO org_enrollment_tokens_created_by_user_id_fkey;
ALTER TABLE org_enrollment_tokens
  RENAME CONSTRAINT enrollment_tokens_role_check
  TO org_enrollment_tokens_role_check;

COMMIT;
