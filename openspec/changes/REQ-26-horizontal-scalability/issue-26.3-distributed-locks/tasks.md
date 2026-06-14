# Tasks: issue-26.3-distributed-locks

- [x] **dl-001**: Package internal/dlock/ → Manager + Lock + WithLock
- [x] **dl-002**: Key hashing stable → HashKey (SHA-256 primeros 8 bytes → int64) + TestHashKey_Stable/_DistinctNames
- [x] **dl-003**: Session-pool aware acquire → conn dedicada del pool retenida hasta Release (no transaction-mode)
- [x] **dl-004**: TryAcquire + Acquire + Release → polling 200ms, ErrTimeout, Release idempotente
- [x] **dl-005**: Métricas → domain_dlock_acquire_total{key,result} + domain_dlock_held_duration_seconds{key} (TestMetrics_AcquireAndHeld) — 2026-06-10
- [x] **test-001**: Concurrent TryAcquire → TestTryAcquire_Concurrent_OnlyOneWins (8 workers, 1 gana) — 2026-06-10
- [x] **test-002**: Conn die auto-release → TestConnDie_AutoReleases (pg_terminate_backend simula crash) — 2026-06-10
- [x] **test-003**: Wait timeout → TestAcquire_WaitTimeout (ErrTimeout tras maxWait) — 2026-06-10
- [x] **test-004**: Different keys no collision → TestHashKey_DistinctNames
- [x] **docs-001**: `docs/operations/distributed-locks.md` + anti-patterns — 2026-06-10
