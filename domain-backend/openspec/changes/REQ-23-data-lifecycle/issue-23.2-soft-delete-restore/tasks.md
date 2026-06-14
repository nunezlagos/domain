# Tasks: issue-23.2-soft-delete-restore

- [x] **sd-001**: Migración soft-delete columns en todas las entidades
- [x] **sd-002**: Re-crear índices hot path con `WHERE deleted_at IS NULL`
- [x] **sd-003**: Adapter del store que aplica filter por defecto
- [x] **sd-004**: Service `internal/service/trash.go` con List/Restore
- [x] **sd-005**: Handler REST trash
- [x] **sd-006**: Cascade soft-delete por service
- [x] **sd-007**: Cron purge diario
- [x] **sd-008**: Borrado S3 attachments en purge
- [x] **sd-009**: Linter SQL/code: queries sin filtro deleted_at → fail
- [x] **test-001**: Soft-delete + restore
- [x] **test-002**: Cascade restore
- [x] **test-003**: Conflict slug
- [x] **test-004**: TTL purge
- [x] **test-005**: Linter detecta query sin filtro
- [x] **docs-001**: `docs/trash.md`
