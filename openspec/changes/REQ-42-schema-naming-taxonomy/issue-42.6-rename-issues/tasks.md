# Tasks: issue-42.6-rename-issues

## Verificación previa (bloqueante)

- [ ] Confirmar nombres reales en la introspección: índices `gherkin_scenarios_pkey`, `gherkin_hu_id_idx`, `gherkin_scenarios_status_idx`; constraint FK `gherkin_scenarios_hu_id_fkey`
- [ ] Confirmar que la tabla NO tiene sequence (PK UUID `gen_random_uuid()`)
- [ ] Confirmar que la tabla NO tiene RLS policy
- [ ] Confirmar que el trigger es `trg_set_updated_at` (genérico, sin sufijo) → NO renombrar
- [ ] Confirmar que la próxima migración libre del lote es `000152`
- [ ] Confirmar que la FK saliente `issue_id → issues(id)` no depende de otra HU del lote

## Migración (`000152`)

- [ ] Crear `000152_rename_gherkin_scenarios.up.sql` con header completo (migration/author/issue/description/breaking/estimated_duration)
- [ ] Up: `ALTER TABLE gherkin_scenarios RENAME TO issue_gherkin_scenarios` dentro de `BEGIN/COMMIT`
- [ ] Up: `ALTER INDEX` x3 (pkey, `gherkin_hu_id_idx`→`issue_gherkin_scenarios_issue_id_idx`, status)
- [ ] Up: `ALTER TABLE ... RENAME CONSTRAINT gherkin_scenarios_hu_id_fkey TO issue_gherkin_scenarios_issue_id_fkey`
- [ ] NO emitir RENAME CONSTRAINT para el pkey (lo renombra el ALTER INDEX)
- [ ] NO renombrar el trigger `trg_set_updated_at`
- [ ] Crear `000152_rename_gherkin_scenarios.down.sql` simétrico (mismos 4 RENAME en orden inverso)
- [ ] Verificar que el par pasa `squawk` (RENAME metadata-only, sin lock prolongado)

## Backend — `internal/service/issue/service.go`

- [ ] Línea 363: `FROM gherkin_scenarios` → `FROM issue_gherkin_scenarios` (MAX position)
- [ ] Línea 368: `INSERT INTO gherkin_scenarios` → `INSERT INTO issue_gherkin_scenarios` (AddScenario)
- [ ] Línea 381: `DELETE FROM gherkin_scenarios` → `DELETE FROM issue_gherkin_scenarios` (RemoveScenario)
- [ ] Línea 411: `FROM gherkin_scenarios` → `FROM issue_gherkin_scenarios` (listScenarios)
- [ ] Línea 425: `FROM gherkin_scenarios` → `FROM issue_gherkin_scenarios` (listScenariosByHuIDs)
- [ ] Línea 462: `INSERT INTO gherkin_scenarios` → `INSERT INTO issue_gherkin_scenarios` (insertScenariosTx)
- [ ] Verificar que NO quedó ningún literal `gherkin_scenarios` en el archivo (`grep -n gherkin_scenarios`)

## Backend — tests

- [ ] `tests/e2e/schema_audit_test.go` línea 47: `"gherkin_scenarios"` → `"issue_gherkin_scenarios"` en `expectedTables`
- [ ] Test de migración: `000152 up` deja `to_regclass('issue_gherkin_scenarios')` no nulo y `gherkin_scenarios` nulo
- [ ] Test de migración: los 3 índices destino existen (`issue_gherkin_scenarios_pkey`, `_issue_id_idx`, `_status_idx`)
- [ ] Test de migración: la constraint `issue_gherkin_scenarios_issue_id_fkey` existe y es FK a `issues`
- [ ] Test de migración: `000152 down` restaura nombres originales
- [ ] Test de integración: `AddScenario` / `listScenarios` funcionan contra la tabla renombrada

## Sabotaje (anti-falsos positivos)

OBLIGATORIO: probar que el test FALLA cuando el rename está incompleto, para descartar un test que pase por casualidad.

- [ ] **Sabotaje 1 — índice legacy olvidado:** aplicar el up renombrando la tabla pero COMENTAR la línea `ALTER INDEX gherkin_hu_id_idx RENAME TO issue_gherkin_scenarios_issue_id_idx`. El test de índices DEBE fallar con `index issue_gherkin_scenarios_issue_id_idx not found` (y `gherkin_hu_id_idx` huérfano sobreviviendo). Confirma que el test realmente inspecciona `pg_indexes` y no solo `to_regclass` de la tabla.

```go
// Sabotaje verificado: con el ALTER INDEX comentado, el test debe romper.
func TestSabotage_GherkinIndexRenameIncompleto(t *testing.T) {
    // Up parcial: tabla renombrada, índice legacy NO renombrado.
    mustExec(t, db, `ALTER TABLE gherkin_scenarios RENAME TO issue_gherkin_scenarios`)
    // (a propósito NO renombramos gherkin_hu_id_idx)

    var n int
    err := db.QueryRow(ctx,
        `SELECT count(*) FROM pg_indexes
         WHERE indexname = 'issue_gherkin_scenarios_issue_id_idx'`).Scan(&n)
    if err != nil {
        t.Fatalf("query pg_indexes: %v", err)
    }
    // El test "verde" exigiría n == 1. Con el sabotaje, n == 0 → DEBE fallar.
    if n == 1 {
        t.Fatal("FALSO POSITIVO: el índice legacy no se renombró pero el test pasó; " +
            "el test no inspecciona pg_indexes correctamente")
    }
    // Confirmación de que el legacy quedó huérfano (evidencia del rename incompleto):
    var legacy int
    _ = db.QueryRow(ctx,
        `SELECT count(*) FROM pg_indexes WHERE indexname = 'gherkin_hu_id_idx'`).Scan(&legacy)
    if legacy != 1 {
        t.Fatal("se esperaba el índice legacy huérfano tras el up parcial")
    }
}
```

- [ ] **Sabotaje 2 — FK rota:** tras el rename, intentar `INSERT INTO issue_gherkin_scenarios (issue_id, ...)` con un `issue_id` UUID inexistente. DEBE fallar con SQLSTATE `23503` (foreign_key_violation). Si pasa, la FK no sobrevivió al rename.
- [ ] **Restaurar:** descomentar el `ALTER INDEX` (sabotaje 1) y reaplicar el up completo → el test de índices vuelve a verde.

## Cierre

- [ ] `go vet ./...` sin warnings
- [ ] `go build ./...` OK
- [ ] `go test ./...` verde (incluye test de migración + audit)
- [ ] `grep -rn "gherkin_scenarios" services/domain-backend/` solo devuelve `issue_gherkin_scenarios` (sin literal legacy suelto)
- [ ] `squawk` verde sobre `000152_rename_gherkin_scenarios.up.sql`
- [ ] Commit en rama `services`: `refactor(schema): rename gherkin_scenarios → issue_gherkin_scenarios (REQ-42.6)`
- [ ] NO git push (repo local-only)
