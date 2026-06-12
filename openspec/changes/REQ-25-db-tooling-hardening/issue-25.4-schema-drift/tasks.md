# Tasks: issue-25.4-schema-drift

- [x] **sd-001**: Image custom `domain/drift-tool` con pg_dump + migrate + diff
- [x] **sd-002**: Script check-drift.sh
- [x] **sd-003**: CronJob k8s 1x/day
- [x] **sd-004**: Migración schema_drift_checks
- [x] **sd-005**: Endpoint GET /admin/db/schema-drift
- [x] **sd-006**: Notif via REQ-20 cuando drift
- [x] **sd-007**: S3 spill diff completo
- [x] **test-001**: Drift simulated detectado
- [x] **test-002**: No drift ok
- [x] **test-003**: Migration dirty detectado
- [x] **docs-001**: `docs/db/schema-drift.md`
