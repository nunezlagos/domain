# Tasks: HU-12.2-repair-actions

## Backend

- [ ] **B1: Crear RepairAction, RepairPlan, RepairReport structs**
      - `internal/doctor/repair.go`

- [ ] **B2: Implementar generateRepairPlan**
      - Detectar issues: missing dirs, non-normalized projects, stale sessions, orphans
      - Construir lista de RepairAction con Execute y Validate

- [ ] **B3: Implementar repairMissingDirs**
      - Query directories from sessions
      - os.Stat + os.MkdirAll por cada uno
      - Reportar created/skipped/failed

- [ ] **B4: Implementar repairNormalizeProjects**
      - Reutilizar NormalizeProject() de HU-08.2
      - UPDATE observations y sessions
      - En transacción

- [ ] **B5: Implementar repairCloseStaleSessions**
      - UPDATE WHERE status='active' AND started_at < -48h
      - Agregar nota de auto-close en summary

- [ ] **B6: Implementar repairOrphanObservations**
      - Solo si --fix-orphans flag está presente
      - Soft delete: SET deleted_at = now

- [ ] **B7: Implementar RepairPlan.Execute**
      - Iterar acciones en orden
      - Si dry-run → solo listar
      - Si no → ejecutar, colectar resultados
      - Errores no detienen otras acciones

- [ ] **B8: Integrar --repair en CLI doctor**
      - Flags: --repair, --dry-run, --fix-orphans, --max-actions
      - Output: tabla de acciones tomadas/fallidas

## Tests

- [ ] **T1: generateRepairPlan detecta missing dirs**
- [ ] **T2: Dry-run no ejecuta acciones**
- [ ] **T3: Stale sessions se cierran correctamente**
- [ ] **T4: Orphans se soft-deleten solo con --fix-orphans**
- [ ] **T5: Repair es idempotente (segunda ejecución no hace nada)**
- [ ] **T6: MaxActions limita reparaciones**
- [ ] **T7: Error parcial no detiene otras acciones**
- [ ] **T8: Sabotaje — no checkear precondición → acción duplicada → test idempotencia falla → restaurar**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/doctor/... -v`
- [ ] Commit: `feat: doctor repair mode with dry-run and actionable fixes`
