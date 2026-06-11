# Tasks: issue-25.7-pgaudit-db-level

- [x] **pa-001**: postgresql.conf shared_preload pgaudit + params
- [x] **pa-002**: Migration CREATE EXTENSION pgaudit + audit_role
- [x] **pa-003**: GRANTs a audit_role en tablas sensibles
- [x] **pa-004**: Logshipper config (promtail/filebeat) routea AUDIT:
- [x] **pa-005**: Storage backend retention 7 años (S3 cold archive)
- [x] **pa-006**: Performance bench
- [x] **test-001**: pgaudit cargado
- [x] **test-002**: DDL captured
- [x] **test-003**: Object audit secrets
- [x] **test-004**: Password redacted
- [x] **test-005**: Shipper route correct
- [x] **docs-001**: `docs/db/pgaudit.md`
