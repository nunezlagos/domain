# Tasks: HU-01.2-observation-crud

## Backend

- [ ] **B1: Crear types.go con las estructuras Observation, ObservationUpdate, ObservationFilter, Candidate**
      ```go
      // Observation — mapeo directo de la tabla observations
      type Observation struct {
          ID              int64
          SessionID       string
          Type            string
          Title           string
          Content         string
          ToolName        string
          Project         string
          Scope           string
          TopicKey        *string
          NormalizedHash  *string
          RevisionCount   int
          DuplicateCount  int
          LastSeenAt      *string
          CreatedAt       string
          UpdatedAt       string
          DeletedAt       *string
      }

      type ObservationUpdate struct {
          Title    *string
          Content  *string
          Type     *string
          Scope    *string
          TopicKey **string   // nil = skip, &nil = set NULL
      }

      type ObservationFilter struct {
          Project        string
          Scope          string
          Type           string
          Limit          int
          Offset         int
          IncludeDeleted bool
          SortDesc       bool
      }

      type Candidate struct {
          ID     int64
          Title  string
          Reason string
      }
      ```

- [ ] **B2: Implementar helpers de normalización y hashing**
      - `normalizeContent(s string) string` — lowercase, trim, collapse whitespace, strip punctuation
      - `computeNormalizedHash(obs Observation) string` — SHA-256 de `project|scope|type|title|normalized_content`
      - `normalizeTitle(s string) string` — lowercase + trim (para hash)

- [ ] **B3: Implementar errores centinela**
      ```go
      var (
          ErrNotFound   = errors.New("observation not found")
          ErrValidation = errors.New("validation error")
          ErrConflict   = errors.New("conflict")
      )
      ```

- [ ] **B4: Implementar AddObservation**
      - Validar title != "" && content != "" → si no, retornar ErrValidation
      - Calcular normalized_hash vía B2
      - Ejecutar findCandidates(db, obs, candidatesOut)
      - Iniciar transacción
        - `INSERT INTO observations (...) VALUES (...)`
        - Si capturePrompt=true y CurrentPrompt != "":
          - `INSERT INTO user_prompts (session_id, content, project) VALUES (?, ?, ?)`
      - Commit transacción
      - Retornar `lastInsertId`
      - Si candidatesOut != nil y hay matches, llenar el slice

- [ ] **B5: Implementar findCandidates**
      ```sql
      SELECT id, title FROM observations
      WHERE normalized_hash = ? AND id != ? AND deleted_at IS NULL
      LIMIT 5
      ```
      - Llenar `[]Candidate` con reason `"exact_hash_match"`
      - No bloquear el insert (solo informativo)

- [ ] **B6: Implementar GetObservation**
      ```sql
      SELECT id, session_id, type, title, content, tool_name, project, scope,
             topic_key, normalized_hash, revision_count, duplicate_count,
             last_seen_at, created_at, updated_at, deleted_at
      FROM observations WHERE id = ? [AND deleted_at IS NULL si !includeDeleted]
      ```
      - `db.QueryRowContext` + scan
      - Si `sql.ErrNoRows` → retornar `ErrNotFound`

- [ ] **B7: Implementar UpdateObservation**
      - Construir `SET` clauses dinámicamente según campos no-nil en `ObservationUpdate`
      - Si title o content cambian, recalcular `normalized_hash`
      - Siempre incrementar `revision_count = revision_count + 1`
      - Siempre actualizar `updated_at = datetime('now')`
      - Solo actualizar observaciones con `deleted_at IS NULL`
      - Si `RowsAffected == 0` → retornar `ErrNotFound`

- [ ] **B8: Implementar DeleteObservation (soft & hard)**
      - **Soft:** `UPDATE observations SET deleted_at = datetime('now') WHERE id = ? AND deleted_at IS NULL`
      - **Hard:** `DELETE FROM observations WHERE id = ?`
      - Ambos: si `RowsAffected == 0` → retornar `ErrNotFound` (ya eliminada o no existe)
      - Hard delete: no hay verificación de deleted_at (se puede hard-delete una soft-deleted)

- [ ] **B9: Implementar RecentObservations**
      - Query builder dinámico con filtros opcionales
      - WHERE `deleted_at IS NULL` por defecto (a menos que `IncludeDeleted = true`)
      - Filtros: project, scope, type (si no son vacíos)
      - ORDER BY `created_at DESC` (o ASC si !SortDesc)
      - LIMIT (default 50 si <= 0) + OFFSET
      - Escanear rows en slice, retornar

- [ ] **B10: Implementar CurrentPrompt global**
      ```go
      var CurrentPrompt string
      ```
      - Package-level variable en `observations.go`
      - No requiere init ni mutex por ahora (single-process)

## Frontend

- [ ] N/A — HU puramente backend

## Tests

- [ ] **T1: TestAddObservation — creación con todos los campos**
      ```go
      func TestAddObservation(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()

          sessionID := insertTestSession(t, db)
          obs := Observation{
              SessionID: sessionID, Type: "fix", Title: "Bug login",
              Content: "El modal no cierra", ToolName: "opencode",
              Project: "Domain", Scope: "project",
              TopicKey: strPtr("auth"),
          }
          var candidates []Candidate
          id, err := AddObservation(db, obs, true, &candidates)
          require.NoError(t, err)
          assert.Greater(t, id, int64(0))
          assert.Empty(t, candidates)

          saved, err := GetObservation(db, id, false)
          require.NoError(t, err)
          assert.Equal(t, "fix", saved.Type)
          assert.Equal(t, "Bug login", saved.Title)
          assert.Equal(t, "El modal no cierra", saved.Content)
          assert.NotEmpty(t, saved.NormalizedHash)
          assert.NotEmpty(t, saved.CreatedAt)
          assert.NotEmpty(t, saved.UpdatedAt)
      }
      ```

- [ ] **T2: TestAddObservationValidation — error sin title ni content**
      ```go
      func TestAddObservationValidation(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()

          obs := Observation{SessionID: "s1", Type: "general"}
          _, err := AddObservation(db, obs, false, nil)
          require.Error(t, err)
          assert.ErrorIs(t, err, ErrValidation)
      }
      ```

- [ ] **T3: TestAddObservationCandidates — conflict detection funciona**
      ```go
      func TestAddObservationCandidates(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          sid := insertTestSession(t, db)

          obs1 := Observation{
              SessionID: sid, Title: "Bug login",
              Content: "El modal no cierra al hacer submit", Project: "Domain",
          }
          id1, err := AddObservation(db, obs1, false, nil)
          require.NoError(t, err)

          obs2 := Observation{
              SessionID: sid, Title: "Bug login",
              Content: "El modal no cierra al hacer submit pero con mas texto",
              Project: "Domain",
          }
          var candidates []Candidate
          id2, err := AddObservation(db, obs2, false, &candidates)
          require.NoError(t, err)
          assert.Greater(t, id2, int64(0))
          assert.NotEmpty(t, candidates)
          assert.Equal(t, id1, candidates[0].ID)
          assert.Equal(t, "exact_hash_match", candidates[0].Reason)
      }
      ```

- [ ] **T4: TestGetObservationNotFound — ID inexistente retorna error**
      ```go
      func TestGetObservationNotFound(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()

          _, err := GetObservation(db, 99999, false)
          require.Error(t, err)
          assert.ErrorIs(t, err, ErrNotFound)
      }
      ```

- [ ] **T5: TestUpdateObservation — actualización parcial de campos**
      ```go
      func TestUpdateObservation(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          sid := insertTestSession(t, db)
          id := insertTestObservation(t, db, sid, "general", "old title", "old content")

          newTitle := "new title"
          newContent := "new content"
          newType := "fix"
          err := UpdateObservation(db, id, ObservationUpdate{
              Title: &newTitle, Content: &newContent, Type: &newType,
          })
          require.NoError(t, err)

          obs, err := GetObservation(db, id, false)
          require.NoError(t, err)
          assert.Equal(t, "new title", obs.Title)
          assert.Equal(t, "new content", obs.Content)
          assert.Equal(t, "fix", obs.Type)
          assert.Equal(t, 2, obs.RevisionCount) // incrementado
      }
      ```

- [ ] **T6: TestUpdateObservationNotFound — actualizar ID inexistente**
      ```go
      func TestUpdateObservationNotFound(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()

          newTitle := "irrelevant"
          err := UpdateObservation(db, 99999, ObservationUpdate{Title: &newTitle})
          require.Error(t, err)
          assert.ErrorIs(t, err, ErrNotFound)
      }
      ```

- [ ] **T7: TestSoftDelete — setea deleted_at y excluye de Recent**
      ```go
      func TestSoftDelete(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          sid := insertTestSession(t, db)
          id := insertTestObservation(t, db, sid, "general", "to delete", "content")

          err := DeleteObservation(db, id, false)
          require.NoError(t, err)

          // get with includeDeleted=true debe mostrar deleted_at no nil
          obs, err := GetObservation(db, id, true)
          require.NoError(t, err)
          assert.NotNil(t, obs.DeletedAt)

          // get with includeDeleted=false debe retornar error
          _, err = GetObservation(db, id, false)
          require.Error(t, err)
          assert.ErrorIs(t, err, ErrNotFound)

          // RecentObservations no debe incluirla
          recent, err := RecentObservations(db, ObservationFilter{Limit: 10})
          require.NoError(t, err)
          for _, r := range recent {
              assert.NotEqual(t, id, r.ID)
          }
      }
      ```

- [ ] **T8: TestHardDelete — elimina físicamente la fila**
      ```go
      func TestHardDelete(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          sid := insertTestSession(t, db)
          id := insertTestObservation(t, db, sid, "general", "to hard delete", "content")

          err := DeleteObservation(db, id, true)
          require.NoError(t, err)

          // GetObservation debe fallar incluso con includeDeleted=true
          _, err = GetObservation(db, id, true)
          require.Error(t, err)
          assert.ErrorIs(t, err, ErrNotFound)
      }
      ```

- [ ] **T9: TestDoubleSoftDelete — segundo soft delete retorna error**
      ```go
      func TestDoubleSoftDelete(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          sid := insertTestSession(t, db)
          id := insertTestObservation(t, db, sid, "general", "delete me", "content")

          err := DeleteObservation(db, id, false)
          require.NoError(t, err)

          err = DeleteObservation(db, id, false)
          require.Error(t, err)
          assert.ErrorIs(t, err, ErrNotFound)
      }
      ```

- [ ] **T10: TestRecentObservations — filtros y límite**
      ```go
      func TestRecentObservations(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          sid := insertTestSession(t, db)

          // Insertar 3 obs de project "Domain" y 2 de "other"
          for i := 0; i < 3; i++ {
              insertTestObservation(t, db, sid, "general", fmt.Sprintf("memo %d", i), "content")
          }
          for i := 0; i < 2; i++ {
              insertTestObservation(t, db, sid, "general", fmt.Sprintf("other %d", i), "content",
                  withProject("other"))
          }

          recent, err := RecentObservations(db, ObservationFilter{
              Project: "Domain", Limit: 10,
          })
          require.NoError(t, err)
          assert.Len(t, recent, 3)

          recent2, err := RecentObservations(db, ObservationFilter{
              Limit: 2,
          })
          require.NoError(t, err)
          assert.Len(t, recent2, 2)
      }
      ```

- [ ] **T11: TestAddObservationForeignKey — session_id inválido propaga error**
      ```go
      func TestAddObservationForeignKey(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()

          obs := Observation{
              SessionID: "nonexistent-session",
              Title:     "orphan",
              Content:   "no parent session",
          }
          _, err := AddObservation(db, obs, false, nil)
          require.Error(t, err)
          assert.Contains(t, err.Error(), "FOREIGN KEY")
      }
      ```

- [ ] **T12: setupTestDB helper + insertTestObservation helper**
      ```go
      func setupTestDB(t *testing.T) *sql.DB {
          t.Helper()
          db, err := store.InitDB(":memory:")
          require.NoError(t, err)
          require.NoError(t, store.RunMigrations(db))
          return db
      }

      type obsOption func(*Observation)

      func withProject(p string) obsOption {
          return func(o *Observation) { o.Project = p }
      }

      func insertTestSession(t *testing.T, db *sql.DB) string {
          t.Helper()
          id := "test-session-" + uuid.NewString()[:8]
          _, err := db.Exec(
              "INSERT INTO sessions (id, project, directory) VALUES (?, 'test', '/tmp')",
              id,
          )
          require.NoError(t, err)
          return id
      }

      func insertTestObservation(t *testing.T, db *sql.DB, sid, typ, title, content string, opts ...obsOption) int64 {
          t.Helper()
          obs := Observation{
              SessionID: sid, Type: typ, Title: title,
              Content: content, Project: "test",
          }
          for _, opt := range opts {
              opt(&obs)
          }
          id, err := AddObservation(db, obs, false, nil)
          require.NoError(t, err)
          return id
      }
      ```

- [ ] **T13: Sabotaje — romper FK reference → confirmar test cae → restaurar**
      1. En el DDL de migración 001, cambiar `session_id TEXT NOT NULL REFERENCES sessions(id)` a `session_id TEXT NOT NULL` (eliminar FK)
      2. Ejecutar `TestAddObservationForeignKey` → debe fallar (inserta con session_id inválido pero no hay FK que lo impida)
      3. Restaurar DDL original con FK
      4. Ejecutar `TestAddObservationForeignKey` nuevamente → debe pasar
      5. Documentar el sabotaje en el test con comentario

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/store/... -v -count=1` — suite completa verde
- [ ] Verificar que no hay fugas de conexiones (goroutines, statements sin cerrar)
- [ ] Commit: `feat: implement observation CRUD with soft-delete and conflict detection`
