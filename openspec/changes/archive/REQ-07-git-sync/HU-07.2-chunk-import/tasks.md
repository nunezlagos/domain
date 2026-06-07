# Tasks: HU-07.2-chunk-import

## Backend

- [ ] **B1: Implementar `Importer` struct**
      ```go
      type Importer struct {
          store     *store.Store
          chunksDir string
          manifest  *Manifest
          stats     ImportStats
      }
      ```

- [ ] **B2: Implementar `readChunk(path string) ([]ChunkRecord, error)`**
      - Abrir archivo, descomprimir gzip
      - Scanner línea por línea
      - Unmarshal cada línea como ChunkRecord
      - Manejar líneas vacías
      - Validar que type sea "observation" o "session"
      - Retornar slice de records

- [ ] **B3: Implementar `applyRecord(tx *sql.Tx, rec ChunkRecord) error`**
      - Switch por `rec.Type`
      - Observation: INSERT OR IGNORE con todos los campos
      - Session: INSERT OR IGNORE con todos los campos
      - Manejar tipo desconocido con error

- [ ] **B4: Implementar `importChunk(entry ChunkEntry) error`**
      - Begin transaction
      - INSERT OR IGNORE en sync_chunks primero
      - readChunk + applyRecord para cada record
      - Commit en éxito, Rollback en error
      - Manejar error de chunk corrupto

- [ ] **B5: Implementar `pendingChunks() ([]ChunkEntry, error)`**
      - Consultar sync_chunks para SHA256s ya importados
      - Filtrar manifest.Chunks a solo los pendientes
      - Preservar orden original del manifest

- [ ] **B6: Implementar `Importer.Run() error`**
      - Cargar manifest
      - Si no existe → error "No manifest found"
      - Obtener pending chunks
      - Iterar con progress output a stderr
      - Acumular stats
      - Print resumen final

- [ ] **B7: Implementar progress output**
      - `[i/N] Importing <sha256[:8]>...` a stderr
      - En error: `Error importing chunk <sha256[:8]>: <reason>`
      - Resumen: `Imported X chunks, skipped Y, Z errors`

- [ ] **B8: Integrar import mode en comando cobra `engram sync`**
      - Flag `--import` setea modo import
      - Instanciar Importer con store y directorio .engram
      - Llamar a `Importer.Run()`

- [ ] **B9: Manejar error de chunk corrupto sin abortar**
      - Recolectar errores en `stats.ErrorDetails`
      - Continuar con siguiente chunk
      - Commit de chunk exitoso aún si otro falló

- [ ] **B10: Validar version de manifest**
      - Si manifest.Version != 1, warning "manifest version X may not be compatible"
      - Intentar import igual (forward compatibility tentativa)

## Tests

- [ ] **T1: TestImportChunk — chunk se importa correctamente**
      ```go
      func TestImportChunk(t *testing.T) {
          db := setupTestDB(t)
          store := NewStore(db)
          imp := NewImporter(store, t.TempDir())
          // Crear un chunk de prueba
          records := []ChunkRecord{{Type: "session", Action: "upsert", Data: map[string]interface{}{"id": "s1", "project": "test", "directory": "/tmp"}, Timestamp: "now"}}
          entry, _ := writeChunk(t.TempDir(), records)
          
          // Re-crear importer con DB nueva
          db2 := setupTestDB(t)
          store2 := NewStore(db2)
          imp2 := NewImporter(store2, t.TempDir())
          // Mover el chunk al dir del importer
          // ...
          
          err := imp2.importChunk(*entry)
          assert.NoError(t, err)
          
          // Verificar que la sesión se insertó
          var count int
          db2.QueryRow("SELECT COUNT(*) FROM sessions WHERE id='s1'").Scan(&count)
          assert.Equal(t, 1, count)
      }
      ```

- [ ] **T2: TestImportIdempotent — segundo import no duplica**
      ```go
      func TestImportIdempotent(t *testing.T) {
          // Importar chunk dos veces
          // Verificar que sync_chunks tiene 1 entry
          // Verificar que sessions tiene 1 registro
      }
      ```

- [ ] **T3: TestPendingChunks — chunks ya importados se skipean**
      ```go
      func TestPendingChunks(t *testing.T) {
          // Crear manifest con 2 chunks
          // Marcar 1 como importado en sync_chunks
          // pendingChunks debe retornar solo 1
      }
      ```

- [ ] **T4: TestCorruptChunk — chunk corrupto no aborta**
      ```go
      func TestCorruptChunk(t *testing.T) {
          // Crear 2 chunks: uno bueno, uno corrupto (no gzip)
          // Import debe procesar el bueno, fallar el corrupto, continuar
          // Stats: 1 imported, 1 error
      }
      ```

- [ ] **T5: TestInsertOrIgnore — registro existente no se duplica**
      ```go
      func TestInsertOrIgnore(t *testing.T) {
          // Insertar observación manualmente
          // Importar chunk con misma observación
          // Verificar COUNT = 1
      }
      ```

- [ ] **T6: TestImportNoManifest — sin manifest da error**
      ```go
      func TestImportNoManifest(t *testing.T) {
          imp := NewImporter(nil, "/nonexistent")
          err := imp.Run()
          assert.Error(t, err)
          assert.Contains(t, err.Error(), "No manifest found")
      }
      ```

- [ ] **T7: TestImportProgress — progress se escribe a stderr**
      ```go
      func TestImportProgress(t *testing.T) {
          // Capturar stderr
          // Importar chunks
          // Verificar que stderr contiene "[1/2]" y "Imported"
      }
      ```

- [ ] **T8: TestImportObservation — observación se importa con todos los campos**
      ```go
      func TestImportObservation(t *testing.T) {
          // Importar chunk con observation
          // Verificar todos los campos en DB
      }
      ```

- [ ] **T9: TestImportSessionOrder — sesiones antes que observations**
      ```go
      func TestImportSessionOrder(t *testing.T) {
          // Manifest debe ordenarse para importar sessions primero
          // O INSERT OR IGNORE maneja FK failure silenciosamente
      }
      ```

- [ ] **T10: Sabotaje — cambiar INSERT OR IGNORE a INSERT → test T5 falla (duplicate PK) → restaurar**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/sync/... -v` suite completa verde
- [ ] Prueba E2E: export en DB1 → import en DB2 → datos idénticos
- [ ] Commit: `feat: add chunk import with INSERT OR IGNORE and sync_chunks tracking`
