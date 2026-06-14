# Tasks: issue-25.1-pgbouncer-pooling

- [x] **pb-001**: Deployment + Service + ConfigMap en helm chart issue-24.1
- [x] **pb-002**: ConfigMap pgbouncer.ini con valores parametrizables
- [x] **pb-003**: Secret userlist auto-generado desde DB creds
- [x] **pb-004**: pgx config app `DescribeExec` mode
- [x] **pb-005**: Exporter sidecar prometheus-pgbouncer-exporter
- [x] **pb-006**: PrometheusRule alertas
- [x] **pb-007**: PDB minAvailable=1
- [x] **pb-008**: Migrar app DB URL a pgbouncer:6432 (var env)
- [x] **pb-009**: Documentar LISTEN/NOTIFY no-soportado y workarounds
- [x] **test-001**: 200 conns app, primary <50
- [x] **test-002**: Failover kill 1 pod
- [x] **test-003**: Prepared stmt funciona
- [x] **test-004**: SET LOCAL en tx (issue-25.5 dependency)
- [x] **test-005**: Exporter metrics
- [x] **docs-001**: `docs/db/pgbouncer.md`
