# Tasks: issue-05.6-http-stats-doctor

## Backend

- [ ] **B1: Definir StatsRepo interface y tipos**
      - Stats, DoctorCheck, Health structs
      - StatsRepo interface con GetStats, RunDoctor, Health

- [ ] **B2: Implementar GetStats en store**
      - COUNT queries para observations (deleted_at IS NULL), sessions, user_prompts
      - COUNT DISTINCT project FROM observations
      - os.Stat del archivo DB (via PRAGMA database_list)
      - MIN/MAX created_at de observations

- [ ] **B3: Handler GET /stats**
      - Llamar `repo.GetStats(ctx)`
      - Responder 200 con Stats JSON

- [ ] **B4: Implementar doctor checks registry**
      - Registrar checks: orphans, fts5, schema, missing_index, wal_mode
      - Cada check recibe (ctx, db, project) y retorna DoctorCheck
      - Si project != "", scoping: agregar `WHERE project = ?` a las queries

- [ ] **B5: Handler GET /doctor**
      - Leer `?project=` y `?check=` de query
      - Si check presente, ejecutar solo ese
      - Ejecutar todos los checks en orden
      - Responder 200 con []DoctorCheck

- [ ] **B6: Implementar Health check**
      - db.PingContext con timeout de 3s
      - Leer version de build info
      - Calcular uptime desde startTime global
      - Status "ok" si ping pasa, "degraded" si falla

- [ ] **B7: Handler GET /health**
      - Llamar `repo.Health(ctx)`
      - Responder 200 siempre (incluso si degraded)
      - Cache-control: no-cache (siempre fresh)

- [ ] **B8: RegisterStatsRoutes**

## Frontend

- [ ] N/A — HU puramente backend

## Tests

- [ ] **T1: TestGetStats — GET /stats con campos**
      ```go
      func TestGetStats(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          createObs(t, srv.URL, sid, "stat1")
          resp, _ := http.Get(srv.URL + "/stats")
          assert.Equal(t, 200, resp.StatusCode)
          var s Stats
          json.NewDecoder(resp.Body).Decode(&s)
          assert.GreaterOrEqual(t, s.TotalObservations, 1)
          assert.GreaterOrEqual(t, s.TotalSessions, 1)
          assert.NotEmpty(t, s.DBPath)
          assert.NotEmpty(t, s.OldestObservation)
      }
      ```

- [ ] **T2: TestDoctor — GET /doctor con checks**
      ```go
      func TestDoctor(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          resp, _ := http.Get(srv.URL + "/doctor")
          assert.Equal(t, 200, resp.StatusCode)
          var checks []DoctorCheck
          json.NewDecoder(resp.Body).Decode(&checks)
          assert.GreaterOrEqual(t, len(checks), 3)
          names := map[string]bool{}
          for _, c := range checks {
              names[c.Name] = true
              assert.Contains(t, []string{"pass", "warn", "fail"}, c.Status)
          }
          assert.True(t, names["orphan_observations"])
          assert.True(t, names["fts5_index"])
      }
      ```

- [ ] **T3: TestDoctorFilter — GET /doctor?check=orphans**
      ```go
      func TestDoctorFilter(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          resp, _ := http.Get(srv.URL + "/doctor?check=orphans")
          var checks []DoctorCheck
          json.NewDecoder(resp.Body).Decode(&checks)
          assert.Len(t, checks, 1)
          assert.Equal(t, "orphan_observations", checks[0].Name)
      }
      ```

- [ ] **T4: TestHealth — GET /health**
      ```go
      func TestHealth(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          resp, _ := http.Get(srv.URL + "/health")
          assert.Equal(t, 200, resp.StatusCode)
          var h Health
          json.NewDecoder(resp.Body).Decode(&h)
          assert.Equal(t, "ok", h.Status)
          assert.NotEmpty(t, h.Version)
          assert.NotEmpty(t, h.Uptime)
          assert.True(t, h.DBAlive)
      }
      ```

- [ ] **T5: TestHealthResponseTime — < 100ms**
      ```go
      func TestHealthResponseTime(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          start := time.Now()
          resp, _ := http.Get(srv.URL + "/health")
          elapsed := time.Since(start)
          assert.Equal(t, 200, resp.StatusCode)
          assert.Less(t, elapsed, 100*time.Millisecond)
      }
      ```

- [ ] **T6: Sabotaje — DB cerrada devuelve degraded**
      1. Cerrar DB, llamar /health
      2. Status debe ser "degraded", DBAlive false
      3. Reabrir DB
      4. Health vuelve a "ok"

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/api/... -v` — suite completa verde
- [ ] Probar manualmente con curl
- [ ] Commit: `feat: HTTP stats, doctor diagnostics, and health check endpoints`
