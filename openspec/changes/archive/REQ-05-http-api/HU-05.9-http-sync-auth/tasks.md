# Tasks: HU-05.9-http-sync-auth

## Backend

- [ ] **B1: Implementar RequireToken middleware**
      ```go
      func RequireToken(next http.Handler) http.Handler {
          return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
              token := os.Getenv("ENGRAM_HTTP_TOKEN")
              if token == "" {
                  writeError(w, apiError{500, "ENGRAM_HTTP_TOKEN not configured"})
                  return
              }
              auth := r.Header.Get("Authorization")
              if !strings.HasPrefix(auth, "Bearer ") {
                  writeError(w, apiError{401, "missing Bearer token"})
                  return
              }
              if strings.TrimPrefix(auth, "Bearer ") != token {
                  writeError(w, apiError{401, "invalid token"})
                  return
              }
              next.ServeHTTP(w, r)
          })
      }
      ```

- [ ] **B2: Definir SyncStatusRepo interface y SyncStatus struct**
      - SyncStatus struct con tags json
      - Reason code constants
      - SyncStatusRepo interface con GetStatus

- [ ] **B3: Implementar GetStatus en store**
      - Delegar en `sync.GetStatus(ctx, db)` (HU-07.3)
      - Fallback: retornar idle con reason_code=0

- [ ] **B4: Handler GET /sync/status**
      - Llamar `repo.GetStatus(ctx)`
      - Responder 200 con SyncStatus JSON

- [ ] **B5: Proteger rutas DELETE en sessions**
      - Crear `RegisterProtectedSessionRoutes(mux, repo, auth)` o envolver handlers individuales
      - DELETE /sessions/{id} pasa por RequireToken

- [ ] **B6: Proteger DELETE /observations/{id}**
      - DELETE con ?hard=true (y soft delete también protegido)

- [ ] **B7: Proteger DELETE /prompts/{id}**
      - DELETE protegido

- [ ] **B8: Asegurar rutas GET /export, POST /import, POST /projects/migrate protegidas**
      - Aplicar middleware en el registro de rutas

- [ ] **B9: Asegurar rutas públicas NO protegidas**
      - GET /health, /stats, /sync/status, /search, /context, /timeline
      - POST /sessions, POST /observations, POST /prompts
      - Verificar que no tienen middleware

- [ ] **B10: RegisterSyncRoutes**

## Frontend

- [ ] N/A — HU puramente backend

## Tests

- [ ] **T1: TestSyncStatus — GET /sync/status**
      ```go
      func TestSyncStatus(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          resp, _ := http.Get(srv.URL + "/sync/status")
          assert.Equal(t, 200, resp.StatusCode)
          var ss SyncStatus
          json.NewDecoder(resp.Body).Decode(&ss)
          assert.Equal(t, "idle", ss.SyncState)
          assert.Equal(t, 0, ss.ReasonCode)
          assert.Equal(t, "none", ss.UpgradeStage)
      }
      ```

- [ ] **T2: TestDeleteRequiresAuth — DELETE sin token → 401**
      ```go
      func TestDeleteRequiresAuth(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          // We need ENGRAM_HTTP_TOKEN set for this test
          t.Setenv("ENGRAM_HTTP_TOKEN", "test-token")
          req, _ := http.NewRequest("DELETE", srv.URL+"/sessions/some-id", nil)
          resp, _ := http.DefaultClient.Do(req)
          assert.Equal(t, 401, resp.StatusCode)
      }
      ```

- [ ] **T3: TestDeleteWithValidToken — DELETE con token → 204**
      ```go
      func TestDeleteWithValidToken(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          t.Setenv("ENGRAM_HTTP_TOKEN", "test-token")
          sid := createTestSessionID(t, srv.URL)
          req, _ := http.NewRequest("DELETE", srv.URL+"/sessions/"+sid, nil)
          req.Header.Set("Authorization", "Bearer test-token")
          resp, _ := http.DefaultClient.Do(req)
          assert.Equal(t, 204, resp.StatusCode)
      }
      ```

- [ ] **T4: TestDeleteWithInvalidToken — 401**
      ```go
      func TestDeleteWithInvalidToken(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          t.Setenv("ENGRAM_HTTP_TOKEN", "test-token")
          req, _ := http.NewRequest("DELETE", srv.URL+"/sessions/some-id", nil)
          req.Header.Set("Authorization", "Bearer wrong-token")
          resp, _ := http.DefaultClient.Do(req)
          assert.Equal(t, 401, resp.StatusCode)
      }
      ```

- [ ] **T5: TestNoTokenConfigured — 500 en rutas protegidas**
      ```go
      func TestNoTokenConfigured(t *testing.T) {
          // Don't set ENGRAM_HTTP_TOKEN
          srv := newTestServer(t)
          defer srv.Close()
          req, _ := http.NewRequest("DELETE", srv.URL+"/sessions/some-id", nil)
          resp, _ := http.DefaultClient.Do(req)
          assert.Equal(t, 500, resp.StatusCode)
      }
      ```

- [ ] **T6: TestPublicRoutesNoAuth — funcionan sin token**
      ```go
      func TestPublicRoutesNoAuth(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          // These must all work without token
          resp1, _ := http.Get(srv.URL + "/health")
          assert.Equal(t, 200, resp1.StatusCode)
          resp2, _ := http.Get(srv.URL + "/stats")
          assert.Equal(t, 200, resp2.StatusCode)
          resp3, _ := http.Get(srv.URL + "/sync/status")
          assert.Equal(t, 200, resp3.StatusCode)
      }
      ```

- [ ] **T7: TestExportRequiresAuth — GET /export sin token**
      ```go
      func TestExportRequiresAuth(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          t.Setenv("ENGRAM_HTTP_TOKEN", "test-token")
          resp, _ := http.Get(srv.URL + "/export?project=test")
          assert.Equal(t, 401, resp.StatusCode)
      }
      ```

- [ ] **T8: Sabotaje — no proteger DELETE**
      1. Comentar `RequireToken` en la ruta DELETE /sessions/{id}
      2. Ejecutar TestDeleteRequiresAuth → FALLA (DELETE retorna 204 sin token)
      3. Restaurar middleware
      4. Test pasa

- [ ] **T9: TestObservationDeleteRequiresAuth**
      ```go
      func TestObservationDeleteRequiresAuth(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          t.Setenv("ENGRAM_HTTP_TOKEN", "test-token")
          req, _ := http.NewRequest("DELETE", srv.URL+"/observations/1", nil)
          resp, _ := http.DefaultClient.Do(req)
          assert.Equal(t, 401, resp.StatusCode)
      }
      ```

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/api/... -v` — suite completa verde
- [ ] Verificar manualmente:
      ```bash
      # Sin token
      curl -X DELETE localhost:7437/sessions/test → 401
      # Con token
      curl -X DELETE -H "Authorization: Bearer mytoken" localhost:7437/sessions/test → 204
      # Sin token en rutas públicas
      curl localhost:7437/health → 200
      curl localhost:7437/sync/status → 200
      ```
- [ ] Commit: `feat: HTTP sync status endpoint and Bearer auth middleware`
