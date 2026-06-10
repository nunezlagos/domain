# Tasks: issue-23.2-soft-delete-restore

- [ ] **sd-001**: Migración soft-delete columns en todas las entidades
- [ ] **sd-002**: Re-crear índices hot path con `WHERE deleted_at IS NULL`
- [ ] **sd-003**: Adapter del store que aplica filter por defecto
- [ ] **sd-004**: Service `internal/service/trash.go` con List/Restore
- [ ] **sd-005**: Handler REST trash
- [ ] **sd-006**: Cascade soft-delete por service
- [ ] **sd-007**: Cron purge diario
- [ ] **sd-008**: Borrado S3 attachments en purge
- [ ] **sd-009**: Linter SQL/code: queries sin filtro deleted_at → fail
- [ ] **test-001**: Soft-delete + restore
- [ ] **test-002**: Cascade restore
- [ ] **test-003**: Conflict slug
- [ ] **test-004**: TTL purge
- [ ] **test-005**: Linter detecta query sin filtro
- [ ] **docs-001**: `docs/trash.md`
