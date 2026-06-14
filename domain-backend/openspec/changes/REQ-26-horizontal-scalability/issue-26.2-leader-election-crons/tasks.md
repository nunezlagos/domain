# Tasks: issue-26.2-leader-election-crons

- [x] **le-001**: Helper `internal/leader/leader.go`
- [x] **le-002**: PgBouncer config session-pool para `_leader`
- [x] **le-003**: Var env `DOMAIN_DATABASE_URL_LEADER`
- [x] **le-004**: Cron worker wrapper aplica leader.Acquire antes
- [x] **le-005**: Heartbeat goroutine 10s
- [x] **le-006**: Métricas `domain_cron_leader{cron}` gauge
- [x] **le-007**: Forced takeover si stale heartbeat
- [x] **le-008**: Aplicar a TODOS los crons existentes (issue-25.2 slow query, issue-25.4 schema drift, issue-25.10 password rotation, issue-18 backup, issue-23.2 trash purge, etc.)
- [x] **test-001**: N goroutines acquire → 1 wins
- [x] **test-002**: Conn die → other acquires
- [x] **test-003**: Heartbeat updates
- [x] **test-004**: Métrica leader correcta
- [x] **test-005**: Stale takeover
- [x] **docs-001**: `docs/operations/leader-election.md`
