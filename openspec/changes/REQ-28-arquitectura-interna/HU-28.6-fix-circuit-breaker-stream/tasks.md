# Tasks: HU-28.6-fix-circuit-breaker-stream

- [ ] **cb-001**: Test RED — stream con error no abre breaker (falla)
- [ ] **cb-002**: Fix `_ = sawError` → `if sawError { cb.recordFailure() }`
- [ ] **cb-003**: Test GREEN — stream con error abre breaker
- [ ] **cb-004**: Sabotaje — recordSuccess en vez de recordFailure, test falla
- [ ] **cb-005**: Suite breaker tests verde (sin regresión en tests existentes)
