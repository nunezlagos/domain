

ALTER ROLE app_user RESET statement_timeout;
ALTER ROLE app_user RESET lock_timeout;
ALTER ROLE app_user RESET idle_in_transaction_session_timeout;
ALTER ROLE app_user CONNECTION LIMIT -1;

ALTER ROLE app_readonly RESET statement_timeout;
ALTER ROLE app_readonly RESET lock_timeout;
ALTER ROLE app_readonly RESET idle_in_transaction_session_timeout;
ALTER ROLE app_readonly CONNECTION LIMIT -1;

ALTER ROLE app_admin RESET statement_timeout;
ALTER ROLE app_admin RESET lock_timeout;
ALTER ROLE app_admin RESET idle_in_transaction_session_timeout;
ALTER ROLE app_admin CONNECTION LIMIT -1;

ALTER ROLE app_migrator RESET statement_timeout;
ALTER ROLE app_migrator RESET lock_timeout;
ALTER ROLE app_migrator RESET idle_in_transaction_session_timeout;
ALTER ROLE app_migrator CONNECTION LIMIT -1;
