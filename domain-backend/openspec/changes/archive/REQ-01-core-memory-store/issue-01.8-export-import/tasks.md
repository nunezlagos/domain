# Tasks: issue-01.8-export-import

## Backend

- [ ] **B1: Crear `internal/store/export.go` con ExportData struct**
      - `ExportData` con slices de Session, Observation, Prompt
      - Tags JSON para cada campo

- [ ] **B2: Implementar `Export(project string) ([]byte, error)`**
      - Query sessions: `SELECT ... FROM sessions ORDER BY id`
      - Query observations: `SELECT ... FROM observations WHERE deleted_at IS NULL` + `AND project = ?` si project != ""
      - Query prompts: `SELECT ... FROM user_prompts` + `WHERE project = ?` si project != ""
      - MarshalIndent con 2 espacios de indentación

- [ ] **B3: Implementar `Import(data []byte) error`**
      - `json.Unmarshal` a ExportData
      - Llamar `validateExportData`
      - `BeginTx` → prepared statements → insertar sessions (OR IGNORE) → observations → prompts → Commit
      - Si cualquier INSERT falla, Rollback

- [ ] **B4: Implementar `validateExportData(data *ExportData) error`**
      - Verificar Sessions != nil
      - Verificar Observations != nil
      - Verificar Prompts != nil
      - Construir set de sessionIDs
      - Verificar que cada observation.SessionID existe en el set
      - Verificar que cada prompt.SessionID existe en el set

- [ ] **B5: Asegurar que los structs Session, Observation, Prompt tengan JSON tags consistentes**
      - Revisar/actualizar structs existentes para que coincidan con el formato de export
      - `json:"id,omitempty"` para campos nullables

## Frontend

- [ ] N/A — HU puramente backend

## Tests

- [ ] **T1: TestExport_Basic**
      ```go
      func TestExport_Basic(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          s := &Store{db: db}
          // insertar session + observation + prompt
          seedTestData(t, db)
          data, err := s.Export(context.Background(), "")
          require.NoError(t, err)
          var export ExportData
          err = json.Unmarshal(data, &export)
          require.NoError(t, err)
          assert.NotEmpty(t, export.Sessions)
          assert.NotEmpty(t, export.Observations)
          assert.NotEmpty(t, export.Prompts)
      }
      ```

- [ ] **T2: TestExport_ProjectFilter**
      ```go
      func TestExport_ProjectFilter(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          s := &Store{db: db}
          s.AddObservation(context.Background(), &Observation{SessionID: "s1", Content: "a", Project: "proj1"})
          s.AddObservation(context.Background(), &Observation{SessionID: "s1", Content: "b", Project: "proj2"})
          data, err := s.Export(context.Background(), "proj1")
          require.NoError(t, err)
          var export ExportData
          json.Unmarshal(data, &export)
          assert.Len(t, export.Observations, 1)
          assert.Equal(t, "proj1", export.Observations[0].Project)
      }
      ```

- [ ] **T3: TestExport_ExcludesSoftDeleted**
      ```go
      func TestExport_ExcludesSoftDeleted(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          s := &Store{db: db}
          id, _ := s.AddObservation(context.Background(), &Observation{SessionID: "s1", Content: "keep me"})
          s.SoftDeleteObservation(context.Background(), id)
          data, _ := s.Export(context.Background(), "")
          var export ExportData
          json.Unmarshal(data, &export)
          assert.Empty(t, export.Observations)
      }
      ```

- [ ] **T4: TestExport_EmptyDB**
      ```go
      func TestExport_EmptyDB(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          s := &Store{db: db}
          data, err := s.Export(context.Background(), "")
          require.NoError(t, err)
          var export ExportData
          json.Unmarshal(data, &export)
          assert.Empty(t, export.Sessions)
          assert.Empty(t, export.Observations)
          assert.Empty(t, export.Prompts)
      }
      ```

- [ ] **T5: TestImport_RoundTrip**
      ```go
      func TestImport_RoundTrip(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          s := &Store{db: db}
          seedTestData(t, db)
          data, _ := s.Export(context.Background(), "")
          // nueva DB vacía
          db2 := setupTestDB(t)
          defer db2.Close()
          s2 := &Store{db: db2}
          err := s2.Import(context.Background(), data)
          require.NoError(t, err)
          // verificar datos
          obs, _ := s2.ListObservations(context.Background(), "", 100, 0)
          assert.NotEmpty(t, obs)
      }
      ```

- [ ] **T6: TestImport_InvalidJSON**
      ```go
      func TestImport_InvalidJSON(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          s := &Store{db: db}
          err := s.Import(context.Background(), []byte(`{"sessions": [invalid}`))
          require.Error(t, err)
          assert.Contains(t, err.Error(), "invalid JSON")
      }
      ```

- [ ] **T7: TestImport_MissingRequiredFields**
      ```go
      func TestImport_MissingRequiredFields(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          s := &Store{db: db}
          err := s.Import(context.Background(), []byte(`{"observations":[],"prompts":[]}`))
          require.Error(t, err)
          assert.Contains(t, err.Error(), "missing required field")
      }
      ```

- [ ] **T8: TestImport_AtomicRollback**
      ```go
      func TestImport_AtomicRollback(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          s := &Store{db: db}
          // JSON con session válida pero observation con session_id inexistente
          badJSON := []byte(`{
              "sessions": [{"id":"s1","project":"test","directory":"/tmp"}],
              "observations": [{"id":1,"session_id":"s2","content":"no session"}],
              "prompts": []
          }`)
          err := s.Import(context.Background(), badJSON)
          require.Error(t, err)
          // la sesión s1 NO debe estar persistida (rollback total)
          var count int
          db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)
          assert.Zero(t, count)
      }
      ```

- [ ] **T9: TestImport_InsertOrIgnoreSessions**
      ```go
      func TestImport_InsertOrIgnoreSessions(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          s := &Store{db: db}
          // session existente
          db.Exec("INSERT INTO sessions (id, project, directory) VALUES ('s1', 'existing', '/old')")
          // import con misma session
          importJSON := []byte(`{
              "sessions": [{"id":"s1","project":"new","directory":"/new"}],
              "observations": [],
              "prompts": []
          }`)
          err := s.Import(context.Background(), importJSON)
          require.NoError(t, err)
          // session no debe haber cambiado
          var project string
          db.QueryRow("SELECT project FROM sessions WHERE id='s1'").Scan(&project)
          assert.Equal(t, "existing", project)
      }
      ```

- [ ] **T10: TestImport_ValidatesBeforeInsert**
      ```go
      func TestImport_ValidatesBeforeInsert(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          s := &Store{db: db}
          // observation referencia session que no está en el JSON
          badJSON := []byte(`{
              "sessions": [],
              "observations": [{"id":1,"session_id":"ghost","content":"oops"}],
              "prompts": []
          }`)
          err := s.Import(context.Background(), badJSON)
          require.Error(t, err)
          assert.Contains(t, err.Error(), "unknown session")
      }
      ```

- [ ] **T11: seedTestData helper**
      ```go
      func seedTestData(t *testing.T, db *sql.DB) {
          t.Helper()
          s := &Store{db: db}
          db.Exec("INSERT INTO sessions (id, project, directory) VALUES ('s1', 'test', '/tmp')")
          s.AddObservation(context.Background(), &Observation{SessionID: "s1", Content: "obs1", Project: "test"})
          s.AddObservation(context.Background(), &Observation{SessionID: "s1", Content: "obs2", Project: "test"})
          s.AddPrompt(context.Background(), "s1", "prompt1", "test")
      }
      ```

- [ ] **T12: Sabotaje — no validar estructura → Import acepta JSON sin sessions → test cae → restaurar**
      1. En Import, comentar `validateExportData(&export)`
      2. Ejecutar TestImport_MissingRequiredFields → debe fallar (no detecta falta de sessions)
      3. Restaurar validación
      4. Ejecutar nuevamente → pasa
      5. Documentar sabotaje

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/store/... -v` — suite completa verde
- [ ] Commit: `feat: add JSON export/import for sessions, observations, and prompts`
