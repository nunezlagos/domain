# Tasks: issue-26.2-leader-election-crons

- [ ] **le-001**: Helper `internal/leader/leader.go`
- [ ] **le-002**: PgBouncer config session-pool para `_leader`
- [ ] **le-003**: Var env `DOMAIN_DATABASE_URL_LEADER`
- [ ] **le-004**: Cron worker wrapper aplica leader.Acquire antes
- [ ] **le-005**: Heartbeat goroutine 10s
- [ ] **le-006**: Métricas `domain_cron_leader{cron}` gauge
- [ ] **le-007**: Forced takeover si stale heartbeat
- [ ] **le-008**: Aplicar a TODOS los crons existentes (issue-25.2 slow query, issue-25.4 schema drift, issue-25.10 password rotation, issue-18 backup, issue-23.2 trash purge, etc.)
- [ ] **test-001**: N goroutines acquire → 1 wins
- [ ] **test-002**: Conn die → other acquires
- [ ] **test-003**: Heartbeat updates
- [ ] **test-004**: Métrica leader correcta
- [ ] **test-005**: Stale takeover
- [ ] **docs-001**: `docs/operations/leader-election.md`
