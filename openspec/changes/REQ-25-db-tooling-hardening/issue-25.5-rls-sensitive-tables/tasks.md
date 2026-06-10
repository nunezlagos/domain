# Tasks: issue-25.5-rls-sensitive-tables

- [ ] **rls-001**: Migrations RLS en 12 tablas (1 migration por tabla o batch)
- [ ] **rls-002**: Policies USING + WITH CHECK
- [ ] **rls-003**: Helper `db.WithOrgTx(ctx, orgID, userID, fn)`
- [ ] **rls-004**: Refactor services para usar helper
- [ ] **rls-005**: Linter test scan queries sin helper
- [ ] **rls-006**: Roles app_admin BYPASSRLS para batch jobs
- [ ] **rls-007**: Tests aislamiento 2 orgs
- [ ] **rls-008**: Performance bench <5% regression
- [ ] **test-001**: Cross-org SELECT bloqueado
- [ ] **test-002**: SET LOCAL correcto
- [ ] **test-003**: Sin SET LOCAL → 0 rows
- [ ] **test-004**: app_admin bypass
- [ ] **test-005**: INSERT WITH CHECK
- [ ] **test-006**: Sabotaje linter detecta
- [ ] **docs-001**: `docs/db/rls.md` con policies + helper usage
