# Tasks: issue-25.8-resource-limits-timeouts

- [x] **rl-001**: Migration ALTER ROLE timeouts + conn limits
- [x] **rl-002**: ConfigMap postgresql.conf calibrado por env
- [x] **rl-003**: ssl certs en K8s Secret + montado
- [x] **rl-004**: pg_hba.conf hardened
- [x] **rl-005**: App pgx URL sslmode=verify-full + sslrootcert
- [x] **rl-006**: Cert expiry monitor + alert 30d antes
- [x] **rl-007**: Documentar override para batch jobs largos (rol específico)
- [x] **test-001**: statement_timeout aborta
- [x] **test-002**: lock_timeout aborta
- [x] **test-003**: idle_in_tx kill
- [x] **test-004**: connection limit
- [x] **test-005**: sslmode=disable rechazado
- [x] **test-006**: Cert hostname mismatch rechazado
- [x] **docs-001**: `docs/db/limits-and-tls.md`
