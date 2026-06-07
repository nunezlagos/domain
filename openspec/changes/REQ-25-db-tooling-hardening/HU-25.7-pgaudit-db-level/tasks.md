# Tasks: HU-25.7-pgaudit-db-level

- [ ] **pa-001**: postgresql.conf shared_preload pgaudit + params
- [ ] **pa-002**: Migration CREATE EXTENSION pgaudit + audit_role
- [ ] **pa-003**: GRANTs a audit_role en tablas sensibles
- [ ] **pa-004**: Logshipper config (promtail/filebeat) routea AUDIT:
- [ ] **pa-005**: Storage backend retention 7 años (S3 cold archive)
- [ ] **pa-006**: Performance bench
- [ ] **test-001**: pgaudit cargado
- [ ] **test-002**: DDL captured
- [ ] **test-003**: Object audit secrets
- [ ] **test-004**: Password redacted
- [ ] **test-005**: Shipper route correct
- [ ] **docs-001**: `docs/db/pgaudit.md`
