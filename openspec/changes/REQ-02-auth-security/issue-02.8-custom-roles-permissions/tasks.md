# Tasks: issue-02.8-custom-roles-permissions

- [ ] **cr-001**: Migración `custom_roles`
- [ ] **cr-002**: Whitelist `internal/auth/rbac/whitelist.go`
- [ ] **cr-003**: Validator JSONB permissions
- [ ] **cr-004**: Service `internal/service/role.go` CRUD
- [ ] **cr-005**: Handlers REST `/organizations/:id/roles`
- [ ] **cr-006**: RBAC middleware extend: probe custom OR built-in
- [ ] **cr-007**: Cache org permissions con LISTEN/NOTIFY
- [ ] **cr-008**: Block edición built-in roles (403)
- [ ] **cr-009**: Block delete con members asignados (409)
- [ ] **test-001**: CRUD + audit
- [ ] **test-002**: Resource-scoped allows/denies
- [ ] **test-003**: Validation 422
- [ ] **test-004**: Cache invalidation cross-node
- [ ] **sabotaje-001**: action "god_mode" → 422
- [ ] **docs-001**: `docs/auth/custom-roles.md`
