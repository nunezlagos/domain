




CREATE OR REPLACE FUNCTION reject_audit_log_modification()
RETURNS TRIGGER AS $$
BEGIN
  RAISE EXCEPTION 'audit_log is append-only: UPDATE/DELETE not allowed'
    USING HINT = 'Insert only, never modify';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS audit_log_immutable ON audit_log;
CREATE TRIGGER audit_log_immutable
  BEFORE UPDATE OR DELETE ON audit_log
  FOR EACH ROW EXECUTE FUNCTION reject_audit_log_modification();
