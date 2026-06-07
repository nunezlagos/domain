# Tasks: HU-05.7-http-projects

## Backend

- [ ] **B1: Definir ProjectRepo interface y tipos**
      - ProjectResult, MigrationResult structs
      - ProjectRepo interface con DetectCurrent, Migrate

- [ ] **B2: Handler GET /project/current**
      - Leer `?cwd=` de query (opcional)
      - Llamar `repo.DetectCurrent(ctx, cwd)`
      - Responder 200 con ProjectResult

- [ ] **B3: Implementar DetectCurrent en store**
      - Si cwd vacío, usar `os.Getwd()`
      - Delegar en `project.Detect(cwd)` (HU-08.1)
      - Si error o no detectado, retornar "default" con confidence "low"

- [ ] **B4: Handler POST /projects/migrate**
      - Parsear body: `{ "from": "old", "to": "new" }`
      - Validar: from y to requeridos, from != to → 400
      - Llamar `repo.Migrate(ctx, from, to)`
      - Responder 200 con MigrationResult

- [ ] **B5: Implementar Migrate en store**
      - BEGIN TRANSACTION
      - UPDATE observations SET project = ? WHERE project = ?
      - UPDATE sessions SET project = ? WHERE project = ?
      - UPDATE user_prompts SET project = ? WHERE project = ?
      - COMMIT
      - Si from == to, retornar error antes de empezar tx

- [ ] **B6: RegisterProjectRoutes**
      ```go
      func RegisterProjectRoutes(mux *http.ServeMux, repo ProjectRepo, requireAuth func(http.Handler) http.Handler) {
          mux.HandleFunc("GET /project/current", handleCurrentProject(repo))
          mux.Handle("POST /projects/migrate", requireAuth(http.HandlerFunc(handleMigrate(repo))))
      }
      ```

## Frontend

- [ ] N/A — HU puramente backend

## Tests

- [ ] **T1: TestCurrentProject — GET /project/current**
      ```go
      func TestCurrentProject(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          resp, _ := http.Get(srv.URL + "/project/current?cwd=/tmp")
          assert.Equal(t, 200, resp.StatusCode)
          var pr ProjectResult
          json.NewDecoder(resp.Body).Decode(&pr)
          assert.Equal(t, "default", pr.Project)
          assert.Equal(t, "low", pr.Confidence)
      }
      ```

- [ ] **T2: TestMigrate — POST /projects/migrate**
      ```go
      func TestMigrate(t *testing.T) {
          srv := newTestServerWithAuth(t)
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          // Create observations in project "old"
          body := fmt.Sprintf(`{"session_id":"%s","content":"migrate me","project":"old"}`, sid)
          http.Post(srv.URL+"/observations", "application/json", strings.NewReader(body))
          // Migrate
          payload := `{"from":"old","to":"new"}`
          req, _ := http.NewRequest("POST", srv.URL+"/projects/migrate",
              strings.NewReader(payload))
          req.Header.Set("Authorization", "Bearer test-token")
          req.Header.Set("Content-Type", "application/json")
          resp, _ := http.DefaultClient.Do(req)
          assert.Equal(t, 200, resp.StatusCode)
          var mr MigrationResult
          json.NewDecoder(resp.Body).Decode(&mr)
          assert.GreaterOrEqual(t, mr.ObservationsMoved, 1)
      }
      ```

- [ ] **T3: TestMigrateSameProject — 400**
      ```go
      func TestMigrateSameProject(t *testing.T) {
          srv := newTestServerWithAuth(t)
          defer srv.Close()
          req, _ := http.NewRequest("POST", srv.URL+"/projects/migrate",
              strings.NewReader(`{"from":"same","to":"same"}`))
          req.Header.Set("Authorization", "Bearer test-token")
          req.Header.Set("Content-Type", "application/json")
          resp, _ := http.DefaultClient.Do(req)
          assert.Equal(t, 400, resp.StatusCode)
      }
      ```

- [ ] **T4: TestMigrateUnauthorized — 401**
      ```go
      func TestMigrateUnauthorized(t *testing.T) {
          srv := newTestServerWithAuth(t)
          defer srv.Close()
          resp, _ := http.Post(srv.URL+"/projects/migrate", "application/json",
              strings.NewReader(`{"from":"a","to":"b"}`))
          assert.Equal(t, 401, resp.StatusCode)
      }
      ```

- [ ] **T5: Sabotaje — migrate sin validar from!=to**
      1. Sacar check de from == to
      2. POST con from="x", to="x" → 200 en vez de 400
      3. Restaurar check
      4. Test pasa

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/api/... -v` — suite completa verde
- [ ] Probar manualmente con curl
- [ ] Commit: `feat: HTTP project resolution and migration endpoints`
