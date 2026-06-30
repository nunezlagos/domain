-- migration: 000154_rename_auth_users_group
-- author: NunezLagos
-- issue: legacy
-- description: renombra tablas de auth/users al grupo/prefijo estandar (multiples ALTER TABLE RENAME)
-- breaking: yes
-- estimated_duration: unknown

BEGIN;






ALTER TABLE otp_codes RENAME TO auth_otp_codes;
ALTER INDEX otp_codes_pkey            RENAME TO auth_otp_codes_pkey;
ALTER INDEX otp_codes_status_idx      RENAME TO auth_otp_codes_status_idx;
ALTER INDEX otp_codes_user_active_idx RENAME TO auth_otp_codes_user_active_idx;

ALTER TABLE auth_otp_codes RENAME CONSTRAINT otp_codes_user_id_fkey TO auth_otp_codes_user_id_fkey;
ALTER POLICY otp_codes_user_isolation ON auth_otp_codes RENAME TO auth_otp_codes_user_isolation;


ALTER TABLE api_keys RENAME TO auth_api_keys;
ALTER INDEX api_keys_pkey           RENAME TO auth_api_keys_pkey;
ALTER INDEX api_keys_status_idx     RENAME TO auth_api_keys_status_idx;
ALTER INDEX api_keys_key_prefix_idx RENAME TO auth_api_keys_key_prefix_idx;
ALTER INDEX api_keys_user_id_idx    RENAME TO auth_api_keys_user_id_idx;

ALTER TABLE auth_api_keys RENAME CONSTRAINT api_keys_user_id_fkey TO auth_api_keys_user_id_fkey;


ALTER TABLE secrets RENAME TO auth_secrets;
ALTER INDEX secrets_pkey       RENAME TO auth_secrets_pkey;
ALTER INDEX secrets_status_idx RENAME TO auth_secrets_status_idx;

ALTER TABLE auth_secrets RENAME CONSTRAINT secrets_created_by_fkey TO auth_secrets_created_by_fkey;


ALTER TABLE invitations RENAME TO auth_invitations;
ALTER INDEX invitations_pkey       RENAME TO auth_invitations_pkey;
ALTER INDEX invitations_status_idx RENAME TO auth_invitations_status_idx;
ALTER INDEX invitations_email_idx  RENAME TO auth_invitations_email_idx;
ALTER INDEX invitations_token_key  RENAME TO auth_invitations_token_key;

ALTER TABLE auth_invitations RENAME CONSTRAINT invitations_status_check             TO auth_invitations_status_check;
ALTER TABLE auth_invitations RENAME CONSTRAINT invitations_role_check               TO auth_invitations_role_check;
ALTER TABLE auth_invitations RENAME CONSTRAINT invitations_invited_by_user_id_fkey  TO auth_invitations_invited_by_user_id_fkey;
ALTER TABLE auth_invitations RENAME CONSTRAINT invitations_accepted_user_id_fkey    TO auth_invitations_accepted_user_id_fkey;













COMMIT;
