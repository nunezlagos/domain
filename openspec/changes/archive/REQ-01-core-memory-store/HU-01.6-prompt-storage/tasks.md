# Tasks: HU-01.6-prompt-storage

## Backend

- [ ] **B1: Crear `internal/store/prompt.go` con Prompt struct y receiver Store**
      - `Prompt` struct con campos: ID, SessionID, Content, Project, CreatedAt
      - Funciones: `AddPrompt`, `GetPrompt`, `ListPrompts`, `DeletePrompt`, `SearchPrompts`

- [ ] **B2: Implementar `AddPrompt(sessionID, content, project string) (int64, error)`**
      - Validar content != "" — error si vacío
      - `INSERT INTO user_prompts` con session_id, content, project
      - Retornar `lastInsertId`
      - Propagación de error por FK violation

- [ ] **B3: Implementar `GetPrompt(id int64) (*Prompt, error)`**
      - `SELECT` por id
      - Retornar nil + error si `sql.ErrNoRows`

- [ ] **B4: Implementar `ListPrompts(project string, limit, offset int) ([]*Prompt, error)`**
      - `WHERE project = ?` si project no está vacío
      - `ORDER BY created_at DESC`
      - `LIMIT ? OFFSET ?`

- [ ] **B5: Implementar `DeletePrompt(id int64) error`**
      - `DELETE FROM user_prompts WHERE id = ?`
      - Verificar filas afectadas (opcional, no crítico)

- [ ] **B6: Implementar `SearchPrompts(query, project string, limit int) ([]*Prompt, error)`**
      - Sanitizar query FTS5 (eliminar caracteres reservados)
      - `SELECT p.* FROM user_prompts p JOIN prompts_fts f ON p.id = f.rowid WHERE prompts_fts MATCH ?`
      - Filtro opcional por project
      - `ORDER BY rank LIMIT ?`

- [ ] **B7: Implementar `capturePrompt(content string)` y `GetCapturedPrompt() string`**
      - Variable global `capturedPrompt` con `sync.Mutex`
      - `capturePrompt` es pública (exportada) para que plugin/MCP la llame
      - `GetCapturedPrompt` consume el buffer (get + clear atómico)

- [ ] **B8: Integrar capturePrompt en AddPrompt**
      - Después de INSERT exitoso, llamar `capturePrompt(content)`
      - Esto asegura que cada prompt guardado está disponible para domain_mem_save

- [ ] **B9: Helper `sanitizeFTS5Query(q string) string`**
      - Regex que elimina `^*"():+-~<>`
      - Tests con caracteres especiales

## Frontend

- [ ] N/A — HU puramente backend

## Tests

- [ ] **T1: TestAddPrompt_Success**
      ```go
      func TestAddPrompt_Success(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          s := &Store{db: db}
          id, err := s.AddPrompt(context.Background(), "session-1", "hello world", "test")
          require.NoError(t, err)
          assert.Greater(t, id, int64(0))
      }
      ```

- [ ] **T2: TestAddPrompt_EmptyContent**
      ```go
      func TestAddPrompt_EmptyContent(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          s := &Store{db: db}
          _, err := s.AddPrompt(context.Background(), "session-1", "", "test")
          require.Error(t, err)
          assert.Contains(t, err.Error(), "content cannot be empty")
      }
      ```

- [ ] **T3: TestAddPrompt_InvalidSession**
      ```go
      func TestAddPrompt_InvalidSession(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          s := &Store{db: db}
          _, err := s.AddPrompt(context.Background(), "nonexistent", "hello", "test")
          require.Error(t, err)
          assert.Contains(t, err.Error(), "FOREIGN KEY")
      }
      ```

- [ ] **T4: TestGetPrompt**
      ```go
      func TestGetPrompt(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          s := &Store{db: db}
          id, _ := s.AddPrompt(context.Background(), "session-1", "hello", "test")
          p, err := s.GetPrompt(context.Background(), id)
          require.NoError(t, err)
          assert.Equal(t, "hello", p.Content)
      }
      ```

- [ ] **T5: TestListPrompts_WithProjectFilter**
      ```go
      func TestListPrompts_WithProjectFilter(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          s := &Store{db: db}
          s.AddPrompt(context.Background(), "s1", "a", "proj1")
          s.AddPrompt(context.Background(), "s1", "b", "proj2")
          s.AddPrompt(context.Background(), "s1", "c", "proj1")
          prompts, err := s.ListPrompts(context.Background(), "proj1", 10, 0)
          require.NoError(t, err)
          assert.Len(t, prompts, 2)
      }
      ```

- [ ] **T6: TestDeletePrompt**
      ```go
      func TestDeletePrompt(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          s := &Store{db: db}
          id, _ := s.AddPrompt(context.Background(), "s1", "hello", "test")
          err := s.DeletePrompt(context.Background(), id)
          require.NoError(t, err)
          _, err = s.GetPrompt(context.Background(), id)
          assert.Error(t, err)
      }
      ```

- [ ] **T7: TestSearchPrompts_FTS5**
      ```go
      func TestSearchPrompts_FTS5(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          s := &Store{db: db}
          s.AddPrompt(context.Background(), "s1", "analiza el archivo main.go", "Domain")
          s.AddPrompt(context.Background(), "s1", "testea el handler de login", "Domain")
          results, err := s.SearchPrompts(context.Background(), "analiza", "", 10)
          require.NoError(t, err)
          assert.Len(t, results, 1)
          assert.Contains(t, results[0].Content, "analiza")
      }
      ```

- [ ] **T8: TestCapturePrompt_Buffer**
      ```go
      func TestCapturePrompt_Buffer(t *testing.T) {
          capturePrompt("test content")
          got := GetCapturedPrompt()
          assert.Equal(t, "test content", got)
          // buffer debe estar vacío después de consumir
          assert.Empty(t, GetCapturedPrompt())
      }
      ```

- [ ] **T9: TestSanitizeFTS5Query**
      ```go
      func TestSanitizeFTS5Query(t *testing.T) {
          cases := []struct{ in, want string }{
              {"hello world", "hello world"},
              {"hello*world", "helloworld"},
              {"\"quote\"", "quote"},
              {"a:b", "ab"},
          }
          for _, tc := range cases {
              assert.Equal(t, tc.want, sanitizeFTS5Query(tc.in))
          }
      }
      ```

- [ ] **T10: TestSearchPrompts_SpecialChars — query con caracteres especiales no crash**
      ```go
      func TestSearchPrompts_SpecialChars(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          s := &Store{db: db}
          s.AddPrompt(context.Background(), "s1", "hello world", "test")
          // caracteres especiales FTS5 no deben causar error
          results, err := s.SearchPrompts(context.Background(), "hello*:\"test\"", "", 10)
          require.NoError(t, err)
          assert.NotEmpty(t, results)
      }
      ```

- [ ] **T11: setupTestDB helper (reutilizar de HU-01.1 o crear local)**
      ```go
      func setupTestDB(t *testing.T) *sql.DB {
          t.Helper()
          db, err := InitDB(":memory:")
          require.NoError(t, err)
          require.NoError(t, RunMigrations(db))
          // insertar session baseline
          db.Exec("INSERT INTO sessions (id, project, directory) VALUES ('session-1', 'test', '/tmp')")
          return db
      }
      ```

- [ ] **T12: Sabotaje — comentar validación content vacío → test cae → restaurar**
      1. En AddPrompt, comentar `if content == "" { return 0, errors.New("...") }`
      2. Ejecutar TestAddPrompt_EmptyContent → debe fallar (inserta content vacío sin error)
      3. Restaurar validación
      4. Ejecutar nuevamente → pasa
      5. Documentar sabotaje

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/store/... -v` — suite completa verde
- [ ] Commit: `feat: add user prompt CRUD with FTS5 search and process-local capture buffer`
