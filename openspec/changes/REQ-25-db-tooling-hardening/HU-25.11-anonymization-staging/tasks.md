# Tasks: HU-25.11-anonymization-staging

- [ ] **an-001**: CLI subcomando anonymize-dump
- [ ] **an-002**: Catalog table→transforms
- [ ] **an-003**: Transforms: email/name/rut/text/phone/token/nullout
- [ ] **an-004**: Walker pgx stream
- [ ] **an-005**: gzip output writer
- [ ] **an-006**: gofakeit deterministic seed
- [ ] **an-007**: RUT generator módulo 11 válido
- [ ] **an-008**: Catalog completeness test (CI fail if PII column without transform)
- [ ] **an-009**: RBAC platform_admin
- [ ] **test-001**: PII emails grep 0
- [ ] **test-002**: PII RUTs grep 0
- [ ] **test-003**: FK preservation
- [ ] **test-004**: Reproducible seed
- [ ] **test-005**: Catalog completeness
- [ ] **docs-001**: `docs/db/anonymization.md`
