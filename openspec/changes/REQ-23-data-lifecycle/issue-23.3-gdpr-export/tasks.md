# Tasks: issue-23.3-gdpr-export

- [ ] **gdpr-001**: Migración export_jobs
- [ ] **gdpr-002**: Service `internal/service/export.go`
- [ ] **gdpr-003**: Exporter package con streaming JSON+ZIP
- [ ] **gdpr-004**: S3 uploader multipart >100MB
- [ ] **gdpr-005**: Signed URL S3 con TTL 24h
- [ ] **gdpr-006**: Email notification con link + checksum
- [ ] **gdpr-007**: Endpoints POST /me/export, GET /me/exports/:id
- [ ] **gdpr-008**: Rate-limit 1/24h
- [ ] **gdpr-009**: README.md template auto-generado
- [ ] **test-001**: Fixture user → ZIP válido
- [ ] **test-002**: Adversarial: user A export no contiene user B
- [ ] **test-003**: Rate-limit 429
- [ ] **test-004**: Signed URL expira en 24h
- [ ] **docs-001**: `docs/gdpr.md`
