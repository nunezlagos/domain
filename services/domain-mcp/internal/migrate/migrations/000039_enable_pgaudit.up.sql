
















DO $$
BEGIN
  BEGIN
    EXECUTE 'CREATE EXTENSION IF NOT EXISTS pgaudit';
  EXCEPTION
    WHEN feature_not_supported THEN
      RAISE NOTICE 'pgaudit not loaded via shared_preload_libraries; skipping';
    WHEN undefined_file THEN
      RAISE NOTICE 'pgaudit extension files not installed on cluster; skipping';
    WHEN OTHERS THEN
      RAISE NOTICE 'pgaudit installation skipped (%): %', SQLSTATE, SQLERRM;
  END;
END$$;





DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pgaudit') THEN
    EXECUTE 'SECURITY LABEL FOR pgaudit ON TABLE audit_log IS ''READ,WRITE''';
    EXECUTE 'SECURITY LABEL FOR pgaudit ON TABLE api_keys IS ''READ,WRITE''';
    EXECUTE 'SECURITY LABEL FOR pgaudit ON TABLE plan_assignments IS ''READ,WRITE''';
  END IF;
END$$;
