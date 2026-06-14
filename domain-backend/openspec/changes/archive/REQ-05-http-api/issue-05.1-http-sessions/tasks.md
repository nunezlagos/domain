# Tasks: issue-05.1-http-sessions

## Backend

- [ ] **B1: Definir interfaz SessionRepo**
      - `internal/api/sessions.go`:
      ```go
      type Session struct {
          ID        string  `json:"id"`
          Project   string  `json:"project"`
          Directory string  `json:"directory"`
          StartedAt string  `json:"started_at"`
          EndedAt   *string `json:"ended_at,omitempty"`
          Summary   *string `json:"summary,omitempty"`
          Status    string  `json:"status"`
      }

      type SessionRepo interface {
          Create(ctx context.Context, project, directory string) (Session, error)
          End(ctx context.Context, id string) (Session, error)
          Recent(ctx context.Context, limit int) ([]Session, error)
          GetByID(ctx context.Context, id string) (Session, error)
          Delete(ctx context.Context, id string) error
          HasObservations(ctx context.Context, id string) (bool, error)
      }
      ```

- [ ] **B2: Implementar helper writeJSON / writeError**
      ```go
      func writeJSON(w http.ResponseWriter, status int, v any) {
          w.Header().Set("Content-Type", "application/json")
          w.WriteHeader(status)
          json.NewEncoder(w).Encode(v)
      }

      type apiError struct {
          Status  int    `json:"-"`
          Message string `json:"error"`
      }

      func writeError(w http.ResponseWriter, err apiError) {
          w.WriteHeader(err.Status)
          json.NewEncoder(w).Encode(err)
      }
      ```

- [ ] **B3: Handler POST /sessions**
      - Parsear body JSON: `{ "project": "...", "directory": "..." }`
      - Validar: `directory` requerido, si no → 400
      - Si `project` vacío → default "default"
      - Llamar `repo.Create(ctx, project, directory)`
      - Responder 201 con Session JSON

- [ ] **B4: Handler POST /sessions/{id}/end**
      - Extraer `{id}` de path
      - Llamar `repo.End(ctx, id)`
      - Si error es `ErrNotFound` → 404
      - Si error es `ErrAlreadyEnded` → 409
      - Success → 200 con Session actualizada

- [ ] **B5: Handler GET /sessions/recent**
      - Leer `?limit=` de query (default 20, max 100)
      - Llamar `repo.Recent(ctx, limit)`
      - Responder 200 con `[]Session`

- [ ] **B6: Handler GET /sessions/{id}**
      - Llamar `repo.GetByID(ctx, id)`
      - NotFound → 404
      - Success → 200

- [ ] **B7: Handler DELETE /sessions/{id}**
      - Llamar `repo.Delete(ctx, id)`
      - NotFound → 404
      - HasObservations → 409 con `{"error": "session has 5 observations, delete refused"}`
      - Success → 204

- [ ] **B8: RegisterSessionRoutes**
      ```go
      func RegisterSessionRoutes(mux *http.ServeMux, repo SessionRepo) {
          mux.HandleFunc("POST /sessions", handleCreateSession(repo))
          mux.HandleFunc("POST /sessions/{id}/end", handleEndSession(repo))
          mux.HandleFunc("GET /sessions/recent", handleRecentSessions(repo))
          mux.HandleFunc("GET /sessions/{id}", handleGetSession(repo))
          mux.HandleFunc("DELETE /sessions/{id}", handleDeleteSession(repo))
      }
      ```

## Frontend

- [ ] N/A — HU puramente backend

## Tests

- [ ] **T1: TestCreateSession — POST 201 con ID generado**
      ```go
      func TestCreateSession(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()

          body := `{"project":"test","directory":"/tmp"}`
          resp, err := http.Post(srv.URL+"/sessions", "application/json", strings.NewReader(body))
          require.NoError(t, err)
          defer resp.Body.Close()

          assert.Equal(t, 201, resp.StatusCode)
          var s Session
          json.NewDecoder(resp.Body).Decode(&s)
          assert.NotEmpty(t, s.ID)
          assert.Equal(t, "test", s.Project)
          assert.Equal(t, "/tmp", s.Directory)
          assert.Equal(t, "active", s.Status)
      }
      ```

- [ ] **T2: TestCreateSessionMissingDirectory — 400**
      ```go
      func TestCreateSessionMissingDirectory(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()

          resp, _ := http.Post(srv.URL+"/sessions", "application/json", strings.NewReader(`{}`))
          assert.Equal(t, 400, resp.StatusCode)
      }
      ```

- [ ] **T3: TestEndSession — POST /end retorna 200 con ended_at**
      ```go
      func TestEndSession(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()

          // create first
          cre := createTestSession(t, srv.URL, "p1", "/tmp")
          // end
          resp, _ := http.Post(srv.URL+"/sessions/"+cre.ID+"/end", "application/json", nil)
          assert.Equal(t, 200, resp.StatusCode)
          var s Session
          json.NewDecoder(resp.Body).Decode(&s)
          assert.Equal(t, "ended", s.Status)
          assert.NotNil(t, s.EndedAt)
      }
      ```

- [ ] **T4: TestEndSessionTwice — 409**
      ```go
      func TestEndSessionTwice(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          cre := createTestSession(t, srv.URL, "p1", "/tmp")
          http.Post(srv.URL+"/sessions/"+cre.ID+"/end", "application/json", nil)
          resp, _ := http.Post(srv.URL+"/sessions/"+cre.ID+"/end", "application/json", nil)
          assert.Equal(t, 409, resp.StatusCode)
      }
      ```

- [ ] **T5: TestEndSessionNotFound — 404**
      ```go
      func TestEndSessionNotFound(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          resp, _ := http.Post(srv.URL+"/sessions/nonexistent/end", "application/json", nil)
          assert.Equal(t, 404, resp.StatusCode)
      }
      ```

- [ ] **T6: TestRecentSessions — GET retorna array ordenado**
      ```go
      func TestRecentSessions(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          createTestSession(t, srv.URL, "p1", "/a")
          createTestSession(t, srv.URL, "p2", "/b")
          resp, _ := http.Get(srv.URL + "/sessions/recent")
          assert.Equal(t, 200, resp.StatusCode)
          var sessions []Session
          json.NewDecoder(resp.Body).Decode(&sessions)
          assert.GreaterOrEqual(t, len(sessions), 2)
      }
      ```

- [ ] **T7: TestRecentSessionsLimit — respeta ?limit=**
      ```go
      func TestRecentSessionsLimit(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          for i := 0; i < 5; i++ {
              createTestSession(t, srv.URL, "p1", "/tmp")
          }
          resp, _ := http.Get(srv.URL + "/sessions/recent?limit=2")
          var sessions []Session
          json.NewDecoder(resp.Body).Decode(&sessions)
          assert.Len(t, sessions, 2)
      }
      ```

- [ ] **T8: TestGetSession — GET {id} retorna 200**
      ```go
      func TestGetSession(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          cre := createTestSession(t, srv.URL, "p1", "/tmp")
          resp, _ := http.Get(srv.URL + "/sessions/" + cre.ID)
          assert.Equal(t, 200, resp.StatusCode)
      }
      ```

- [ ] **T9: TestGetSessionNotFound — 404**
      ```go
      func TestGetSessionNotFound(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          resp, _ := http.Get(srv.URL + "/sessions/nonexistent")
          assert.Equal(t, 404, resp.StatusCode)
      }
      ```

- [ ] **T10: TestDeleteSession — 204**
      ```go
      func TestDeleteSession(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          cre := createTestSession(t, srv.URL, "p1", "/tmp")
          req, _ := http.NewRequest("DELETE", srv.URL+"/sessions/"+cre.ID, nil)
          resp, _ := http.DefaultClient.Do(req)
          assert.Equal(t, 204, resp.StatusCode)
      }
      ```

- [ ] **T11: TestDeleteSessionWithObservations — 409**
      ```go
      func TestDeleteSessionWithObservations(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          cre := createTestSession(t, srv.URL, "p1", "/tmp")
          createTestObservation(t, srv.URL, cre.ID, "test obs")
          req, _ := http.NewRequest("DELETE", srv.URL+"/sessions/"+cre.ID, nil)
          resp, _ := http.DefaultClient.Do(req)
          assert.Equal(t, 409, resp.StatusCode)
      }
      ```

- [ ] **T12: TestDeleteSessionNotFound — 404**
      ```go
      func TestDeleteSessionNotFound(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          req, _ := http.NewRequest("DELETE", srv.URL+"/sessions/nonexistent", nil)
          resp, _ := http.DefaultClient.Do(req)
          assert.Equal(t, 404, resp.StatusCode)
      }
      ```

- [ ] **T13: Sabotaje — eliminar check de HasObservations**
      1. Comentar el `if count > 0 { return ErrSessionHasObservations }` en Delete
      2. Ejecutar TestDeleteSessionWithObservations → FALLA (DELETE pasa, esperaba 409)
      3. Restaurar código
      4. Test pasa nuevamente
      5. Documentar sabotaje

- [ ] **T14: TestCreateSessionDefaults — project default**
      ```go
      func TestCreateSessionDefaults(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          resp, _ := http.Post(srv.URL+"/sessions", "application/json",
              strings.NewReader(`{"directory":"/tmp"}`))
          var s Session
          json.NewDecoder(resp.Body).Decode(&s)
          assert.Equal(t, "default", s.Project)
      }
      ```

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/api/... -v` — suite completa verde
- [ ] Probar manualmente con curl:
      ```bash
      curl -s -X POST localhost:7437/sessions -d '{"project":"test","directory":"/tmp"}'
      curl -s localhost:7437/sessions/recent
      curl -s -X POST localhost:7437/sessions/{id}/end
      curl -s -X DELETE localhost:7437/sessions/{id}
      ```
- [ ] Commit: `feat: HTTP sessions CRUD endpoints`
