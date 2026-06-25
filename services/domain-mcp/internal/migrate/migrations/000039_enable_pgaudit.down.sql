
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pgaudit') THEN
    EXECUTE 'SECURITY LABEL FOR pgaudit ON TABLE audit_log IS NULL';
    EXECUTE 'SECURITY LABEL FOR pgaudit ON TABLE api_keys IS NULL';
    EXECUTE 'SECURITY LABEL FOR pgaudit ON TABLE plan_assignments IS NULL';
  END IF;
END$$;



