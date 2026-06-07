# Tasks: HU-25.8-resource-limits-timeouts

- [ ] **rl-001**: Migration ALTER ROLE timeouts + conn limits
- [ ] **rl-002**: ConfigMap postgresql.conf calibrado por env
- [ ] **rl-003**: ssl certs en K8s Secret + montado
- [ ] **rl-004**: pg_hba.conf hardened
- [ ] **rl-005**: App pgx URL sslmode=verify-full + sslrootcert
- [ ] **rl-006**: Cert expiry monitor + alert 30d antes
- [ ] **rl-007**: Documentar override para batch jobs largos (rol específico)
- [ ] **test-001**: statement_timeout aborta
- [ ] **test-002**: lock_timeout aborta
- [ ] **test-003**: idle_in_tx kill
- [ ] **test-004**: connection limit
- [ ] **test-005**: sslmode=disable rechazado
- [ ] **test-006**: Cert hostname mismatch rechazado
- [ ] **docs-001**: `docs/db/limits-and-tls.md`
