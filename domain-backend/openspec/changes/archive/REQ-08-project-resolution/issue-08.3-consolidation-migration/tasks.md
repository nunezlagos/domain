# Tasks: issue-08.3-consolidation-migration

## Backend

- [ ] **B1: Implementar ConsolidateProjects()**
      - `internal/project/consolidate.go`
      - UPDATE sessions, observations, user_prompts en transacción
      - Validar from != to
      - Validar que from existe (SELECT COUNT)
      - Soporte dry-run (SELECT COUNT en lugar de UPDATE)

- [ ] **B2: Implementar POST /api/projects/migrate**
      - Handler en `internal/api/projects.go`
      - Parsear JSON body: {from, to, dry_run}
      - Llamar ConsolidateProjects()
      - Retornar ConsolidateResult como JSON

- [ ] **B3: Implementar `engram projects consolidate` CLI**
      - Flags: `--from`, `--to`, `--dry-run`, `--interactive`
      - Modo interactive: listar candidatos, confirmar
      - Llamar ConsolidateProjects() o API según modo

- [ ] **B4: Implementar `mem_merge_projects` tool function**
      - Registrar tool en MCP server
      - Ejecutar ConsolidateProjects y retornar resultado

- [ ] **B5: Implementar query de candidates para interactive mode**
      - `SELECT project, COUNT(*) as obs_count FROM observations GROUP BY project HAVING COUNT(*) > 0 ORDER BY obs_count DESC`
      - Detectar variantes similares con NormalizeProject (issue-08.2)

## Frontend

- [ ] **F1: Botón o comando de consolidate en TUI**
      - Opción en project context menu

## Tests

- [ ] **T1: ConsolidateProjects migra datos correctamente**
      ```go
      func TestConsolidateMigrates(t *testing.T) {
          db := setupTestDB(t)
          seedProject(db, "old-name", 5)
          seedProject(db, "new-name", 3)
          result, err := ConsolidateProjects(context.Background(), db, "old-name", "new-name", ConsolidateOpts{})
          assert.NoError(t, err)
          assert.True(t, result.Success)
          assert.Equal(t, int64(5), result.MigratedObservations)
          // Verify: old-name ya no tiene observaciones
          var count int
          db.QueryRow("SELECT COUNT(*) FROM observations WHERE project = 'old-name'").Scan(&count)
          assert.Equal(t, 0, count)
      }
      ```

- [ ] **T2: Dry-run no modifica datos**
      ```go
      func TestConsolidateDryRun(t *testing.T) {
          db := setupTestDB(t)
          seedProject(db, "old-name", 5)
          result, err := ConsolidateProjects(context.Background(), db, "old-name", "new-name", ConsolidateOpts{DryRun: true})
          assert.NoError(t, err)
          assert.True(t, result.DryRun)
          assert.Equal(t, int64(5), result.MigratedObservations)
          // Verify: old-name todavía tiene sus datos
          var count int
          db.QueryRow("SELECT COUNT(*) FROM observations WHERE project = 'old-name'").Scan(&count)
          assert.Equal(t, 5, count)
      }
      ```

- [ ] **T3: Error si origen = destino**
      ```go
      func TestConsolidateSameProject(t *testing.T) {
          _, err := ConsolidateProjects(context.Background(), nil, "same", "same", ConsolidateOpts{})
          assert.ErrorContains(t, err, "must be different")
      }
      ```

- [ ] **T4: Error si origen no existe**
      ```go
      func TestConsolidateSourceNotFound(t *testing.T) {
          db := setupTestDB(t)
          _, err := ConsolidateProjects(context.Background(), db, "nonexistent", "new-name", ConsolidateOpts{})
          assert.ErrorContains(t, err, "source project not found")
      }
      ```

- [ ] **T5: POST /api/projects/migrate handler**
      ```go
      func TestMigrateEndpoint(t *testing.T) {
          // Setup router + DB
          body := `{"from":"old-name","to":"new-name"}`
          req := httptest.NewRequest("POST", "/api/projects/migrate", strings.NewReader(body))
          w := httptest.NewRecorder()
          router.ServeHTTP(w, req)
          assert.Equal(t, 200, w.Code)
          // Parse response
      }
      ```

- [ ] **T6: Sabotaje — no hacer Commit en transacción → datos no persisten → restaurar**
      1. Eliminar `tx.Commit()` del código
      2. TestConsolidateMigrates falla (datos no persisten)
      3. Restaurar `tx.Commit()`
      4. Test pasa

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/project/... -v` — suite completa verde
- [ ] Commit: `feat: project consolidation and migration with HTTP endpoint and CLI`
