# Tasks: HU-07.3-sync-status

## Backend

- [ ] **B1: Implementar `StatusReport` struct con todos los campos**
      ```go
      type StatusReport struct {
          Local      LocalCounts
          Remote     RemoteCounts
          LastExport string
          Health     *HealthReport
      }
      type LocalCounts struct {
          Observations  int
          Sessions      int
          ActiveSessions int
      }
      type RemoteCounts struct {
          Records int
          Chunks  int
      }
      type HealthReport struct {
          Status   string   // "healthy", "degraded", "corrupt"
          Total    int
          Verified int
          Missing  int
          Corrupt  int
          Details  []string
      }
      ```

- [ ] **B2: Implementar queries de conteo local en store**
      - `CountObservations() (int, error)` — `SELECT COUNT(*) FROM observations WHERE deleted_at IS NULL`
      - `CountSessions() (int, error)` — `SELECT COUNT(*) FROM sessions`
      - `CountActiveSessions() (int, error)` — `SELECT COUNT(*) FROM sessions WHERE status = 'active'`

- [ ] **B3: Implementar `remoteCounts(manifest *Manifest) RemoteCounts`**
      - Sumar `recordCount` de todos los chunks
      - Si no hay manifest, retornar RemoteCounts{0, 0}

- [ ] **B4: Implementar `HealthCheck(chunksDir string, manifest *Manifest) *HealthReport`**
      - Iterar manifest.Chunks
      - Verificar existencia de cada archivo
      - Verificar SHA-256 (descomprimir, re-hash)
      - Clasificar: healthy, degraded (missing), corrupt (SHA mismatch)

- [ ] **B5: Implementar `StatusReport.Format() string`**
      - Output formateado con separadores
      - Si es TTY, usar bold/colors
      - Mostrar diff local vs remote
      - Mostrar health status con emoji/texto
      - Sugerir acción según diff

- [ ] **B6: Integrar en comando cobra `engram sync --status`**
      - Flag `--status` (puede ser string para compatibilidad futura o bool)
      - Ejecutar status queries y mostrar resultado
      - Exit code 0 si healthy, 1 si corrupt/degraded

- [ ] **B7: Manejar caso sin manifest**
      - Mostrar "No manifest found. Run 'engram sync' to create one."
      - Mostrar solo local counts
      - Health = nil (no aplica)

- [ ] **B8: Agregar diff y sugerencia de acción**
      - Si local > remote → sugerir export
      - Si remote > local → sugerir import
      - Si igual → "In sync"
      - Diff como texto descriptivo

## Tests

- [ ] **T1: TestLocalCounts — conteos locales desde store mock**
      ```go
      func TestLocalCounts(t *testing.T) {
          db := setupTestDB(t)
          store := NewStore(db)
          // Insertar algunos registros
          seedTestData(db)
          
          sr := StatusReport{}
          sr.Local.Observations, _ = store.CountObservations()
          sr.Local.Sessions, _ = store.CountSessions()
          sr.Local.ActiveSessions, _ = store.CountActiveSessions()
          
          assert.Equal(t, 5, sr.Local.Observations)
          assert.Equal(t, 2, sr.Local.Sessions)
          assert.Equal(t, 1, sr.Local.ActiveSessions)
      }
      ```

- [ ] **T2: TestRemoteCounts — suma recordCount del manifest**
      ```go
      func TestRemoteCounts(t *testing.T) {
          m := &Manifest{
              Chunks: []ChunkEntry{
                  {RecordCount: 100},
                  {RecordCount: 200},
              },
          }
          rc := remoteCounts(m)
          assert.Equal(t, 300, rc.Records)
          assert.Equal(t, 2, rc.Chunks)
      }
      ```

- [ ] **T3: TestHealthCheckHealthy — todos los chunks OK**
      ```go
      func TestHealthCheckHealthy(t *testing.T) {
          dir := t.TempDir()
          // Crear chunks reales
          records := []ChunkRecord{{Type: "test", Action: "upsert", Data: "d", Timestamp: "now"}}
          entry1, _ := writeChunk(dir, records)
          entry2, _ := writeChunk(dir, records)
          m := &Manifest{Chunks: []ChunkEntry{*entry1, *entry2}}
          
          hr := HealthCheck(dir, m)
          assert.Equal(t, "healthy", hr.Status)
          assert.Equal(t, 2, hr.Verified)
      }
      ```

- [ ] **T4: TestHealthCheckMissing — chunk faltante detectado**
      ```go
      func TestHealthCheckMissing(t *testing.T) {
          dir := t.TempDir()
          m := &Manifest{Chunks: []ChunkEntry{{SHA256: "nonexistent", Size: 100, RecordCount: 1, ExportedAt: "now"}}}
          hr := HealthCheck(dir, m)
          assert.Equal(t, "degraded", hr.Status)
          assert.Equal(t, 1, hr.Missing)
      }
      ```

- [ ] **T5: TestHealthCheckCorrupt — SHA mismatch detectado**
      ```go
      func TestHealthCheckCorrupt(t *testing.T) {
          dir := t.TempDir()
          // Escribir chunk con contenido que no coincide con SHA
          os.WriteFile(filepath.Join(dir, "abc123.jsonl.gz"), []byte("corrupted"), 0644)
          m := &Manifest{Chunks: []ChunkEntry{{SHA256: "abc123", Size: 10, RecordCount: 1, ExportedAt: "now"}}}
          hr := HealthCheck(dir, m)
          assert.Equal(t, "corrupt", hr.Status)
          assert.Equal(t, 1, hr.Corrupt)
      }
      ```

- [ ] **T6: TestStatusOutput — formato de output contiene secciones**
      ```go
      func TestStatusOutput(t *testing.T) {
          sr := StatusReport{
              Local: LocalCounts{Observations: 1234, Sessions: 56, ActiveSessions: 3},
              Remote: RemoteCounts{Records: 800, Chunks: 5},
              LastExport: "2026-06-01T14:30:00Z",
              Health: &HealthReport{Status: "healthy", Total: 5, Verified: 5},
          }
          output := sr.Format()
          assert.Contains(t, output, "1234")
          assert.Contains(t, output, "800")
          assert.Contains(t, output, "healthy")
          assert.Contains(t, output, "Last export")
      }
      ```

- [ ] **T7: TestStatusNoManifest — sin manifest solo muestra local**
      ```go
      func TestStatusNoManifest(t *testing.T) {
          sr := StatusReport{
              Local: LocalCounts{Observations: 100, Sessions: 5},
          }
          output := sr.Format()
          assert.Contains(t, output, "100")
          assert.Contains(t, output, "No manifest found")
      }
      ```

- [ ] **T8: TestDiffSuggestion — diff sugiere acción correcta**
      ```go
      func TestDiffSuggestion(t *testing.T) {
          // local > remote → suggest export
          // remote > local → suggest import
          // equal → "In sync"
      }
      ```

- [ ] **T9: TestCountActiveSessions — solo sesiones activas**
      ```go
      func TestCountActiveSessions(t *testing.T) {
          // Insertar sessions con status mixto
          // Verificar CountActiveSessions
      }
      ```

- [ ] **T10: Sabotaje — romper query de conteo → test T1 falla → restaurar**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/sync/... -v` suite completa verde
- [ ] Verificar status real: `engram sync --status` output legible
- [ ] Commit: `feat: add sync status command with local/remote counts and manifest health check`
