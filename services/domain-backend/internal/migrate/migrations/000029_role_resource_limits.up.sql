-- migration: role_resource_limits
-- author: mnunez@saargo.com
-- issue: HU-25.8
-- description: Timeouts y connection limits per role (defense contra runaway queries)
-- breaking: false
-- estimated_duration: <1s
--
-- Hardening defense-in-depth:
--   * statement_timeout: query individual no puede colgar el primary
--   * lock_timeout: espera de lock no bloquea workers
--   * idle_in_transaction_session_timeout: tx zombie libera locks
--   * CONNECTION LIMIT: cap por role evita exhaustion
--
-- app_migrator NO recibe timeout (migrations grandes pueden tardar minutos).
-- app_admin sin timeout (batch jobs nocturnos, vacuum, reindex).
-- app_readonly statement_timeout más permisivo (reporting queries).

ALTER ROLE app_user SET statement_timeout = '30s';
ALTER ROLE app_user SET lock_timeout = '10s';
ALTER ROLE app_user SET idle_in_transaction_session_timeout = '60s';
ALTER ROLE app_user CONNECTION LIMIT 200;

ALTER ROLE app_readonly SET statement_timeout = '120s';
ALTER ROLE app_readonly SET lock_timeout = '10s';
ALTER ROLE app_readonly SET idle_in_transaction_session_timeout = '60s';
ALTER ROLE app_readonly CONNECTION LIMIT 50;

ALTER ROLE app_admin SET statement_timeout = '0';
ALTER ROLE app_admin SET lock_timeout = '30s';
ALTER ROLE app_admin SET idle_in_transaction_session_timeout = '300s';
ALTER ROLE app_admin CONNECTION LIMIT 10;

ALTER ROLE app_migrator SET statement_timeout = '0';
ALTER ROLE app_migrator SET lock_timeout = '60s';
ALTER ROLE app_migrator SET idle_in_transaction_session_timeout = '0';
ALTER ROLE app_migrator CONNECTION LIMIT 5;
