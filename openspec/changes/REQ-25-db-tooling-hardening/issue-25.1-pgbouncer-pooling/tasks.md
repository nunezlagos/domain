# Tasks: issue-25.1-pgbouncer-pooling

- [ ] **pb-001**: Deployment + Service + ConfigMap en helm chart issue-24.1
- [ ] **pb-002**: ConfigMap pgbouncer.ini con valores parametrizables
- [ ] **pb-003**: Secret userlist auto-generado desde DB creds
- [ ] **pb-004**: pgx config app `DescribeExec` mode
- [ ] **pb-005**: Exporter sidecar prometheus-pgbouncer-exporter
- [ ] **pb-006**: PrometheusRule alertas
- [ ] **pb-007**: PDB minAvailable=1
- [ ] **pb-008**: Migrar app DB URL a pgbouncer:6432 (var env)
- [ ] **pb-009**: Documentar LISTEN/NOTIFY no-soportado y workarounds
- [ ] **test-001**: 200 conns app, primary <50
- [ ] **test-002**: Failover kill 1 pod
- [ ] **test-003**: Prepared stmt funciona
- [ ] **test-004**: SET LOCAL en tx (issue-25.5 dependency)
- [ ] **test-005**: Exporter metrics
- [ ] **docs-001**: `docs/db/pgbouncer.md`
