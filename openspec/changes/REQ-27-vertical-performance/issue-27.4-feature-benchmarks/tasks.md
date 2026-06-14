# Tasks: issue-27.4-feature-benchmarks

> Nota 2026-06-11: el repo es LOCAL-ONLY (regla git.md) — no hay remote ni
> GitHub Actions ejecutándose. Todo lo que depende de CI (workflow, PR
> comments, history) queda DIFERIDO hasta que el usuario abra remote; los
> workflows están especificados pero no son verificables hoy.

- [x] **fb-001**: Benchmarks por feature crítico → *_bench_test.go en 4 paquetes hot-path (cursor encode/decode/hash, etag, dlock hashkey, anonymizer fakers+redact) con baselines documentadas
- [ ] **fb-002**: GitHub Actions workflow → DIFERIDO (local-only, sin CI ejecutable)
- [x] **fb-003**: benchstat → workflow local documentado en docs/benchmarks/README.md (correr suite + comparar)
- [ ] **fb-004**: Threshold 10% regression → DIFERIDO con fb-002
- [ ] **fb-005**: Override PR body parser → DIFERIDO con fb-002
- [ ] **fb-006**: SLO targets file + check → DIFERIDO con fb-002
- [ ] **fb-007**: History CSV → DIFERIDO con fb-002
- [ ] **fb-008**: PR comment → DIFERIDO con fb-002
- [ ] **test-001/002/003**: Regression/override/SLO → DIFERIDOS con fb-002 (son tests del pipeline CI)
- [x] **docs-001**: docs/benchmarks/README.md → cómo correr, baselines (AMD Ryzen 7 3700X), áreas pendientes (search FTS, agent_run, flow_run — benchmarks heavy con testcontainers para CI weekly futuro)
