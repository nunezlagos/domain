# Tasks: issue-07.1-chunk-export-manifest

## Backend

- [ ] **B1: Crear paquete `internal/sync/` con estructura**
      - `exporter.go` — lógica de export
      - `chunk.go` — chunk creation, content addressing
      - `manifest.go` — manifest.json management

- [ ] **B2: Implementar `ChunkRecord` y `Manifest` structs**
      ```go
      type ChunkRecord struct {
          Type      string      `json:"type"`
          Action    string      `json:"action"`
          Data      interface{} `json:"data"`
          Timestamp string      `json:"timestamp"`
      }
      type Manifest struct {
          Version   int           `json:"version"`
          CreatedAt string        `json:"createdAt"`
          Chunks    []ChunkEntry  `json:"chunks"`
      }
      type ChunkEntry struct {
          SHA256      string `json:"sha256"`
          Size        int64  `json:"size"`
          RecordCount int    `json:"recordCount"`
          ExportedAt  string `json:"exportedAt"`
      }
      ```

- [ ] **B3: Implementar `writeChunk(dir string, records []ChunkRecord) (*ChunkEntry, error)`**
      - Marshal JSONL a bytes
      - Compute SHA-256
      - Gzip comprimir
      - Escribir a temp file + rename atómico
      - Retornar ChunkEntry

- [ ] **B4: Implementar `loadManifest(dir string) (*Manifest, error)` y `saveManifest`**
      - Leer/escribir `.engram/manifest.json`
      - Crear manifest default si no existe
      - Ordenar chunks por exportedAt desc
      - Atomic write (temp + rename)

- [ ] **B5: Implementar `Exporter.Run(project string, all bool) error`**
      - Crear `.engram/chunks/` si no existe
      - Cargar manifest existente
      - Determinar `since` (último export o 30 días atrás)
      - Query observaciones y sesiones desde `since`
      - Particionar en batches de ~5000 records
      - Escribir chunks
      - Actualizar manifest

- [ ] **B6: Implementar queries de export en store**
      - `GetObservationsForExport(since time.Time, project string, limit, offset int) ([]Observation, error)`
      - `GetSessionsForExport(since time.Time, project string, limit, offset int) ([]Session, error)`
      - Project filter: `WHERE project = ?` (si no está vacío)
      - Time filter: `WHERE (updated_at > ? OR created_at > ?)`

- [ ] **B7: Implementar chunk size limit (500KB comprimido)**
      - Monitorear tamaño del buffer gzip durante escritura
      - Si excede 500KB, partir en chunks más pequeños
      - Record individual >500KB va en chunk propio

- [ ] **B8: Implementar comando cobra `engram sync` con flags**
      - `--project`, `--all`, `--import`
      - Validación de flags mutuamente excluyentes
      - Mensaje de progreso en stderr
      - Exit code 0 en éxito, 1 en error

- [ ] **B9: Implementar export incremental**
      - Leer `exportedAt` del chunk más reciente en manifest
      - Usar eso como `since`
      - Si no hay exports previos, usar 30 días atrás (o --all)
      - Si --all, since = zero time

- [ ] **B10: Limpieza de temp files**
      - Al iniciar export, eliminar cualquier `.tmp` en `.engram/chunks/`
      - Registrar en log si se encuentran

## Tests

- [ ] **T1: TestWriteChunk — chunk se escribe con SHA-256 correcto**
      ```go
      func TestWriteChunk(t *testing.T) {
          dir := t.TempDir()
          records := []ChunkRecord{{Type: "observation", Action: "upsert", Data: map[string]string{"title": "test"}, Timestamp: "now"}}
          entry, err := writeChunk(dir, records)
          assert.NoError(t, err)
          // Verificar que el archivo existe con ese nombre
          _, err = os.Stat(filepath.Join(dir, entry.SHA256+".jsonl.gz"))
          assert.NoError(t, err)
      }
      ```

- [ ] **T2: TestChunkContentAddressing — nombre = hash del contenido**
      ```go
      func TestChunkContentAddressing(t *testing.T) {
          records := []ChunkRecord{{Type: "test", Action: "upsert", Data: "data", Timestamp: "now"}}
          entry, _ := writeChunk(t.TempDir(), records)
          // Releer, descomprimir, re-hash
          data, _ := os.ReadFile(filepath.Join(t.TempDir(), entry.SHA256+".jsonl.gz"))
          gr, _ := gzip.NewReader(bytes.NewReader(data))
          decompressed, _ := io.ReadAll(gr)
          hash := sha256.Sum256(decompressed)
          assert.Equal(t, entry.SHA256, hex.EncodeToString(hash[:]))
      }
      ```

- [ ] **T3: TestManifestCreate — manifest se crea con chunk entry**
      ```go
      func TestManifestCreate(t *testing.T) {
          dir := t.TempDir()
          entry := &ChunkEntry{SHA256: "abc", Size: 100, RecordCount: 5, ExportedAt: "now"}
          m := &Manifest{Version: 1, CreatedAt: "now"}
          m.Chunks = append(m.Chunks, *entry)
          os.MkdirAll(filepath.Join(dir, "chunks"), 0755)
          saveManifest(dir, m)
          loaded, err := loadManifest(dir)
          assert.NoError(t, err)
          assert.Equal(t, 1, len(loaded.Chunks))
          assert.Equal(t, "abc", loaded.Chunks[0].SHA256)
      }
      ```

- [ ] **T4: TestManifestBackup — backup se crea antes de sobrescribir**
      ```go
      func TestManifestBackup(t *testing.T) {
          // saveManifest debe hacer backup del manifest previo
      }
      ```

- [ ] **T5: TestProjectFilter — --project filtra registros**
      ```go
      func TestProjectFilter(t *testing.T) {
          // Mock store con observaciones de 2 proyectos
          // Exporter con project="myapp"
          // Verificar que chunks solo contienen records de myapp
      }
      ```

- [ ] **T6: TestAllFlag — --all exporta sin límite**
      ```go
      func TestAllFlag(t *testing.T) {
          // Exporter.Run con all=true
          // Verificar que since es zero time
      }
      ```

- [ ] **T7: TestIncrementalExport — solo nuevos registros**
      ```go
      func TestIncrementalExport(t *testing.T) {
          // Primer export → todos los registros
          // Agregar un nuevo registro
          // Segundo export → solo el nuevo registro
      }
      ```

- [ ] **T8: TestChunkSizeLimit — chunk no excede 500KB**
      ```go
      func TestChunkSizeLimit(t *testing.T) {
          // Generar muchos records hasta superar 500KB comprimido
          // Verificar que se parte en múltiples chunks
      }
      ```

- [ ] **T9: TestJSONLValidity — cada línea en chunk es JSON válido**
      ```go
      func TestJSONLValidity(t *testing.T) {
          // Escribir chunk, leer, descomprimir
          // Cada línea debe parsear como JSON
      }
      ```

- [ ] **T10: Sabotaje — cambiar SHA-256 después de escribir → hash mismatch → test T2 falla → restaurar**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/sync/... -v` suite completa verde
- [ ] Verificar export real: `engram sync --project test` produce .engram/chunks/
- [ ] Commit: `feat: add chunk export with SHA-256 content addressing and manifest tracking`
