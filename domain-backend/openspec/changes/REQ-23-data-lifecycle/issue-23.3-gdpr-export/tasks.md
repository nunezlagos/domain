# Tasks: issue-23.3-gdpr-export

- [x] **gdpr-001**: Migración export_jobs
- [x] **gdpr-002**: Service `internal/service/export.go`
- [x] **gdpr-003**: Exporter package con streaming JSON+ZIP
- [x] **gdpr-004**: S3 uploader multipart >100MB
- [x] **gdpr-005**: Signed URL S3 con TTL 24h
- [x] **gdpr-006**: Email notification con link + checksum
- [x] **gdpr-007**: Endpoints POST /me/export, GET /me/exports/:id
- [x] **gdpr-008**: Rate-limit 1/24h
- [x] **gdpr-009**: README.md template auto-generado
- [x] **test-001**: Fixture user → ZIP válido
- [x] **test-002**: Adversarial: user A export no contiene user B
- [x] **test-003**: Rate-limit 429
- [x] **test-004**: Signed URL expira en 24h
- [x] **docs-001**: `docs/gdpr.md`
