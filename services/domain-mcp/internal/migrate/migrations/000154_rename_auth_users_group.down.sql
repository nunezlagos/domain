











BEGIN;






ALTER TABLE auth_invitations RENAME TO invitations;
ALTER INDEX auth_invitations_pkey       RENAME TO invitations_pkey;
ALTER INDEX auth_invitations_status_idx RENAME TO invitations_status_idx;
ALTER INDEX auth_invitations_email_idx  RENAME TO invitations_email_idx;
ALTER INDEX auth_invitations_token_key  RENAME TO invitations_token_key;

ALTER TABLE invitations RENAME CONSTRAINT auth_invitations_status_check            TO invitations_status_check;
ALTER TABLE invitations RENAME CONSTRAINT auth_invitations_role_check              TO invitations_role_check;
ALTER TABLE invitations RENAME CONSTRAINT auth_invitations_invited_by_user_id_fkey TO invitations_invited_by_user_id_fkey;
ALTER TABLE invitations RENAME CONSTRAINT auth_invitations_accepted_user_id_fkey   TO invitations_accepted_user_id_fkey;


ALTER TABLE auth_secrets RENAME TO secrets;
ALTER INDEX auth_secrets_pkey       RENAME TO secrets_pkey;
ALTER INDEX auth_secrets_status_idx RENAME TO secrets_status_idx;

ALTER TABLE secrets RENAME CONSTRAINT auth_secrets_created_by_fkey TO secrets_created_by_fkey;


ALTER TABLE auth_api_keys RENAME TO api_keys;
ALTER INDEX auth_api_keys_pkey           RENAME TO api_keys_pkey;
ALTER INDEX auth_api_keys_status_idx     RENAME TO api_keys_status_idx;
ALTER INDEX auth_api_keys_key_prefix_idx RENAME TO api_keys_key_prefix_idx;
ALTER INDEX auth_api_keys_user_id_idx    RENAME TO api_keys_user_id_idx;

ALTER TABLE api_keys RENAME CONSTRAINT auth_api_keys_user_id_fkey TO api_keys_user_id_fkey;


ALTER TABLE auth_otp_codes RENAME TO otp_codes;
ALTER INDEX auth_otp_codes_pkey            RENAME TO otp_codes_pkey;
ALTER INDEX auth_otp_codes_status_idx      RENAME TO otp_codes_status_idx;
ALTER INDEX auth_otp_codes_user_active_idx RENAME TO otp_codes_user_active_idx;

ALTER TABLE otp_codes RENAME CONSTRAINT auth_otp_codes_user_id_fkey TO otp_codes_user_id_fkey;
ALTER POLICY auth_otp_codes_user_isolation ON otp_codes RENAME TO otp_codes_user_isolation;

COMMIT;
