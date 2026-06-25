
















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
