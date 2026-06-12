# Tasks: issue-25.10-db-secrets-rotation

- [x] **dr-001**: Subcomando `domain-mcp rotate-db-password --role X`
- [x] **dr-002**: 4 CronJobs k8s staggered
- [x] **dr-003**: ESO/AWS-SM integration optional
- [x] **dr-004**: K8s plain Secret support
- [x] **dr-005**: kubectl wait for rollout complete
- [x] **dr-006**: PgBouncer userlist regenerator + RELOAD
- [x] **dr-007**: Rollback en failure
- [x] **dr-008**: Audit log entries
- [x] **test-001**: Manual rotation zero-downtime
- [x] **test-002**: Cron scheduled cada role
- [x] **test-003**: Failure → rollback
- [x] **test-004**: ESO sync
- [x] **test-005**: PgBouncer reload
- [x] **docs-001**: `docs/db/password-rotation.md`
