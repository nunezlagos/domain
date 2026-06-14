# Tasks: issue-02.1-session-start-end

## Backend

- [ ] **B1: Crear `internal/store/session.go` con tipos base**
      - Struct `Session` con campos ID, Project, Directory, StartedAt, EndedAt, Summary, Status
      - Tipo `SessionStatus` con constantes `StatusActive`, `StatusCompleted`
      - Variables centinela `ErrSessionNotFound`, `ErrSessionAlreadyEnded`
      - Struct `SessionStore` con campo `db *sql.DB`

- [ ] **B2: Implementar `SessionStore.Start(ctx, id, project, directory)`**
      - Si `id` es vacío, generar UUID v7 con `github.com/google/uuid`
      - `INSERT INTO sessions (id, project, directory, started_at, status) VALUES (?, ?, ?, ?, 'active')`
      - Retornar `*Session` completo (re-query o construir desde datos)

- [ ] **B3: Implementar `SessionStore.End(ctx, id, summary?)`**
      - BEGIN TRANSACTION
      - `SELECT status FROM sessions WHERE id = ?` — si no rows → `ErrSessionNotFound`
      - Si status ya es `completed` → `ErrSessionAlreadyEnded`
      - `UPDATE sessions SET status='completed', ended_at=?, summary=COALESCE(?, summary) WHERE id=?`
      - COMMIT
      - Si rowsAffected == 0 después del UPDATE → retornar error (capa extra de safety)

- [ ] **B4: Implementar `SessionStore.Status(ctx, id)`**
      - `SELECT id, project, directory, started_at, ended_at, summary, status FROM sessions WHERE id = ?`
      - Si no rows → `ErrSessionNotFound`
      - Mapear fila a struct `Session`

- [ ] **B5: Agregar dependencia `github.com/google/uuid`**
      - `go get github.com/google/uuid`
      - Verificar UUID v7: `uuid.Must(uuid.NewV7())` si disponible; fallback a `uuid.New()`
      - Pinner versión en `go.mod`

## Frontend (TUI)

- [ ] **B6: Crear componente `SessionBadge` en `internal/tui/badge.go`**
      - Función `SessionBadge(status string) string` con lipgloss styling
      - Verde para active, gris para completed, gris oscuro para unknown
      - Test unitario del render

## Tests

- [ ] **T1: TestSessionStart — crear sesión y verificar campos**
      ```go
      func TestSessionStart(t *testing.T) {
          db := setupTestDB(t)
          store := NewSessionStore(db)
          sess, err := store.Start(ctx, "", "Domain", "/home/user/memoria")
          require.NoError(t, err)
          assert.NotEmpty(t, sess.ID)
          assert.Equal(t, "Domain", sess.Project)
          assert.Equal(t, "/home/user/memoria", sess.Directory)
          assert.False(t, sess.StartedAt.IsZero())
          assert.Equal(t, StatusActive, sess.Status)
          assert.Nil(t, sess.EndedAt)
      }
      ```

- [ ] **T2: TestSessionEnd — cerrar sesión activa**
      ```go
      func TestSessionEnd(t *testing.T) {
          db := setupTestDB(t)
          store := NewSessionStore(db)
          sess, _ := store.Start(ctx, "s1", "p1", "/d1")
          summary := "trabajo completado"
          err := store.End(ctx, sess.ID, &summary)
          require.NoError(t, err)
          ended, _ := store.Status(ctx, sess.ID)
          assert.Equal(t, StatusCompleted, ended.Status)
          assert.NotNil(t, ended.EndedAt)
          assert.Equal(t, summary, *ended.Summary)
      }
      ```

- [ ] **T3: TestSessionEndNotFound — error con id inexistente**
      ```go
      func TestSessionEndNotFound(t *testing.T) {
          db := setupTestDB(t)
          store := NewSessionStore(db)
          err := store.End(ctx, "nonexistent", nil)
          assert.ErrorIs(t, err, ErrSessionNotFound)
      }
      ```

- [ ] **T4: TestSessionEndAlreadyEnded — doble End da error**
      ```go
      func TestSessionEndAlreadyEnded(t *testing.T) {
          db := setupTestDB(t)
          store := NewSessionStore(db)
          sess, _ := store.Start(ctx, "s1", "p1", "/d1")
          require.NoError(t, store.End(ctx, sess.ID, nil))
          err := store.End(ctx, sess.ID, nil)
          assert.ErrorIs(t, err, ErrSessionAlreadyEnded)
      }
      ```

- [ ] **T5: TestSessionFullCycle — Start → Status → End → Status**
      ```go
      func TestSessionFullCycle(t *testing.T) {
          db := setupTestDB(t)
          store := NewSessionStore(db)
          sess, _ := store.Start(ctx, "", "Domain", "/tmp")
          st1, _ := store.Status(ctx, sess.ID)
          assert.Equal(t, StatusActive, st1.Status)
          require.NoError(t, store.End(ctx, sess.ID, nil))
          st2, _ := store.Status(ctx, sess.ID)
          assert.Equal(t, StatusCompleted, st2.Status)
          assert.NotNil(t, st2.EndedAt)
      }
      ```

- [ ] **T6: TestSessionStartWithCustomID — usar id provisto**
      ```go
      func TestSessionStartWithCustomID(t *testing.T) {
          db := setupTestDB(t)
          store := NewSessionStore(db)
          sess, err := store.Start(ctx, "custom-id", "p1", "/d1")
          require.NoError(t, err)
          assert.Equal(t, "custom-id", sess.ID)
      }
      ```

- [ ] **T7: TestSessionBadgeRender — componente TUI**
      ```go
      func TestSessionBadgeRender(t *testing.T) {
          active := SessionBadge("active")
          assert.Contains(t, active, "active")
          completed := SessionBadge("completed")
          assert.Contains(t, completed, "completed")
      }
      ```

- [ ] **T8: setupTestDB helper para REQ-02**
      ```go
      func setupTestDB(t *testing.T) *sql.DB {
          t.Helper()
          db, err := store.InitDB(":memory:")
          require.NoError(t, err)
          require.NoError(t, store.RunMigrations(db))
          return db
      }
      ```

- [ ] **T9: Sabotaje — id vacío sin generación de UUID**
      1. En `Start()`, comentar la línea que genera UUID
      2. Ejecutar TestSessionStart → debe fallar (id vacío inserta NULL violando PK)
      3. Restaurar generación de UUID
      4. Ejecutar TestSessionStart nuevamente → debe pasar
      5. Documentar el sabotaje

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/store/... ./internal/tui/... -v` — suite completa verde
- [ ] Verificar que `go.mod` incluye `github.com/google/uuid`
- [ ] Commit: `feat: implement session start, end, and status tracking`
