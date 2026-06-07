# Tasks: HU-25.10-db-secrets-rotation

- [ ] **dr-001**: Subcomando `domain-mcp rotate-db-password --role X`
- [ ] **dr-002**: 4 CronJobs k8s staggered
- [ ] **dr-003**: ESO/AWS-SM integration optional
- [ ] **dr-004**: K8s plain Secret support
- [ ] **dr-005**: kubectl wait for rollout complete
- [ ] **dr-006**: PgBouncer userlist regenerator + RELOAD
- [ ] **dr-007**: Rollback en failure
- [ ] **dr-008**: Audit log entries
- [ ] **test-001**: Manual rotation zero-downtime
- [ ] **test-002**: Cron scheduled cada role
- [ ] **test-003**: Failure → rollback
- [ ] **test-004**: ESO sync
- [ ] **test-005**: PgBouncer reload
- [ ] **docs-001**: `docs/db/password-rotation.md`
