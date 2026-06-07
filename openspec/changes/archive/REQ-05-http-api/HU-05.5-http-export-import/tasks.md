# Tasks: HU-05.5-http-export-import

## Backend

- [ ] **B1: Definir ExportRepo interface y tipos**
      - ExportPayload, ImportResult structs
      - ExportRepo interface con ExportProject, ImportProject

- [ ] **B2: Handler GET /export**
      - Leer `?project=` de query (required)
      - Validar project no vacío → 400
      - Llamar `repo.ExportProject(ctx, project)`
      - Responder 200 con ExportPayload JSON

- [ ] **B3: Implementar ExportProject en store**
      - Query sessions: `SELECT ... FROM sessions WHERE project = ?`
      - Query observations: `SELECT ... FROM observations WHERE project = ? AND deleted_at IS NULL`
      - Query prompts: `SELECT ... FROM user_prompts WHERE project = ?`
      - Construir ExportPayload con metadatos (exported_at, source, version)

- [ ] **B4: Handler POST /import**
      - Parsear body como ExportPayload
      - Llamar `repo.ImportProject(ctx, payload)`
      - Si error → 500 con detalle
      - Success → 200 con ImportResult

- [ ] **B5: Implementar ImportProject en store**
      - BEGIN TRANSACTION
      - INSERT OR IGNORE sessions (evitar duplicados por ID)
      - INSERT observations (sin OR IGNORE — si falla, rollback)
      - INSERT prompts
      - COMMIT (si todo ok) o ROLLBACK (si algún error)
      - Retornar ImportResult con conteos

- [ ] **B6: Agregar auth middleware a rutas**
      - Envolver handlers con `RequireToken` middleware
      - El middleware lee `ENGRAM_HTTP_TOKEN` de env
      - Verifica `Authorization: Bearer <token>`

- [ ] **B7: RegisterExportRoutes**
      ```go
      func RegisterExportRoutes(mux *http.ServeMux, repo ExportRepo, requireAuth func(http.Handler) http.Handler) {
          mux.Handle("GET /export", requireAuth(http.HandlerFunc(handleExport(repo))))
          mux.Handle("POST /import", requireAuth(http.HandlerFunc(handleImport(repo))))
      }
      ```

## Frontend

- [ ] N/A — HU puramente backend

## Tests

- [ ] **T1: TestExport — GET /export con datos**
      ```go
      func TestExport(t *testing.T) {
          srv := newTestServerWithAuth(t) // server with ENGRAM_HTTP_TOKEN set
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          createObs(t, srv.URL, sid, "export test")
          req, _ := http.NewRequest("GET", srv.URL+"/export?project=default", nil)
          req.Header.Set("Authorization", "Bearer test-token")
          resp, _ := http.DefaultClient.Do(req)
          assert.Equal(t, 200, resp.StatusCode)
          var ep ExportPayload
          json.NewDecoder(resp.Body).Decode(&ep)
          assert.Equal(t, "default", ep.Project)
          assert.Equal(t, "Domain", ep.Source)
          assert.NotEmpty(t, ep.ExportedAt)
      }
      ```

- [ ] **T2: TestExportNoProject — 400**
      ```go
      func TestExportNoProject(t *testing.T) {
          srv := newTestServerWithAuth(t)
          defer srv.Close()
          req, _ := http.NewRequest("GET", srv.URL+"/export", nil)
          req.Header.Set("Authorization", "Bearer test-token")
          resp, _ := http.DefaultClient.Do(req)
          assert.Equal(t, 400, resp.StatusCode)
      }
      ```

- [ ] **T3: TestExportUnauthorized — 401**
      ```go
      func TestExportUnauthorized(t *testing.T) {
          srv := newTestServerWithAuth(t)
          defer srv.Close()
          resp, _ := http.Get(srv.URL + "/export?project=test")
          assert.Equal(t, 401, resp.StatusCode)
      }
      ```

- [ ] **T4: TestImport — POST /import con payload válido**
      ```go
      func TestImport(t *testing.T) {
          srv := newTestServerWithAuth(t)
          defer srv.Close()
          payload := ExportPayload{
              Project: "imported",
              Sessions: []Session{
                  {ID: "s-import-1", Project: "imported", Directory: "/tmp", Status: "active"},
              },
              Observations: []Observation{
                  {SessionID: "s-import-1", Content: "imported obs", Project: "imported"},
              },
          }
          body, _ := json.Marshal(payload)
          req, _ := http.NewRequest("POST", srv.URL+"/import", bytes.NewReader(body))
          req.Header.Set("Authorization", "Bearer test-token")
          req.Header.Set("Content-Type", "application/json")
          resp, _ := http.DefaultClient.Do(req)
          assert.Equal(t, 200, resp.StatusCode)
          var result ImportResult
          json.NewDecoder(resp.Body).Decode(&result)
          assert.Equal(t, 1, result.SessionsImported)
          assert.Equal(t, 1, result.ObservationsImported)
      }
      ```

- [ ] **T5: TestImportAtomic — rollback en error**
      ```go
      func TestImportAtomic(t *testing.T) {
          srv := newTestServerWithAuth(t)
          defer srv.Close()
          payload := ExportPayload{
              Project: "atomic",
              Sessions: []Session{
                  {ID: "s-atomic", Project: "atomic", Directory: "/tmp", Status: "active"},
              },
              Observations: []Observation{
                  {SessionID: "nonexistent-session", Content: "bad FK", Project: "atomic"},
              },
          }
          body, _ := json.Marshal(payload)
          req, _ := http.NewRequest("POST", srv.URL+"/import", bytes.NewReader(body))
          req.Header.Set("Authorization", "Bearer test-token")
          req.Header.Set("Content-Type", "application/json")
          resp, _ := http.DefaultClient.Do(req)
          assert.Equal(t, 500, resp.StatusCode)
          // Verify session was NOT created (rollback)
          req2, _ := http.NewRequest("GET", srv.URL+"/sessions/s-atomic", nil)
          resp2, _ := http.DefaultClient.Do(req2)
          assert.Equal(t, 404, resp2.StatusCode)
      }
      ```

- [ ] **T6: TestImportIdempotent — INSERT OR IGNORE sessions**
      ```go
      func TestImportIdempotent(t *testing.T) {
          srv := newTestServerWithAuth(t)
          defer srv.Close()
          payload := ExportPayload{
              Project: "idemp",
              Sessions: []Session{
                  {ID: "s-idem", Project: "idemp", Directory: "/tmp", Status: "active"},
              },
              Observations: []Observation{
                  {SessionID: "s-idem", Content: "obs1", Project: "idemp"},
              },
          }
          body, _ := json.Marshal(payload)
          // Import twice
          for i := 0; i < 2; i++ {
              req, _ := http.NewRequest("POST", srv.URL+"/import", bytes.NewReader(body))
              req.Header.Set("Authorization", "Bearer test-token")
              req.Header.Set("Content-Type", "application/json")
              http.DefaultClient.Do(req)
          }
          // Should have only 1 session
          req, _ := http.NewRequest("GET", srv.URL+"/sessions/s-idem", nil)
          resp, _ := http.DefaultClient.Do(req)
          assert.Equal(t, 200, resp.StatusCode)
      }
      ```

- [ ] **T7: Sabotaje — INSERT OR REPLACE en vez de OR IGNORE**
      1. Cambiar a `INSERT OR REPLACE INTO sessions`
      2. Import dos veces con distintos directory
      3. Segunda import sobreescribe primera (esperábamos preservar)
      4. Restaurar a INSERT OR IGNORE
      5. Test pasa

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/api/... -v` — suite completa verde
- [ ] Probar manualmente con curl y token
- [ ] Commit: `feat: HTTP export/import endpoints with atomic transactions`
