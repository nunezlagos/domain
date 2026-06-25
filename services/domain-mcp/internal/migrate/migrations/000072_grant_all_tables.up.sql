


















DO $grants$
DECLARE
  tbl text;
  seq text;
BEGIN

  FOR tbl IN
    SELECT quote_ident(table_name)
    FROM information_schema.tables
    WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
  LOOP
    EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON public.%s TO app_user', tbl);
    EXECUTE format('GRANT ALL ON public.%s TO app_admin', tbl);
    EXECUTE format('GRANT SELECT ON public.%s TO app_readonly', tbl);
  END LOOP;


  FOR seq IN
    SELECT quote_ident(sequence_name)
    FROM information_schema.sequences
    WHERE sequence_schema = 'public'
  LOOP
    EXECUTE format('GRANT USAGE, SELECT, UPDATE ON SEQUENCE public.%s TO app_user', seq);
    EXECUTE format('GRANT ALL ON SEQUENCE public.%s TO app_admin', seq);
  END LOOP;


  FOR tbl IN
    SELECT quote_ident(table_name)
    FROM information_schema.views
    WHERE table_schema = 'public'
  LOOP
    EXECUTE format('GRANT SELECT ON public.%s TO app_user', tbl);
    EXECUTE format('GRANT SELECT ON public.%s TO app_readonly', tbl);
    EXECUTE format('GRANT SELECT ON public.%s TO app_admin', tbl);
  END LOOP;
END
$grants$;





ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT ALL ON TABLES TO app_admin;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT SELECT ON TABLES TO app_readonly;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO app_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT ALL ON SEQUENCES TO app_admin;
