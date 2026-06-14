# Tasks: issue-02.8-custom-roles-permissions

- [x] **cr-001**: Migración `custom_roles`
- [x] **cr-002**: Whitelist `internal/auth/rbac/whitelist.go`
- [x] **cr-003**: Validator JSONB permissions
- [x] **cr-004**: Service `internal/service/role.go` CRUD
- [x] **cr-005**: Handlers REST `/organizations/:id/roles`
- [x] **cr-006**: RBAC middleware extend: probe custom OR built-in
- [x] **cr-007**: Cache org permissions con LISTEN/NOTIFY
- [x] **cr-008**: Block edición built-in roles (403)
- [x] **cr-009**: Block delete con members asignados (409)
- [x] **test-001**: CRUD + audit
- [x] **test-002**: Resource-scoped allows/denies
- [x] **test-003**: Validation 422
- [x] **test-004**: Cache invalidation cross-node
- [x] **sabotaje-001**: action "god_mode" → 422
- [x] **docs-001**: `docs/auth/custom-roles.md`
