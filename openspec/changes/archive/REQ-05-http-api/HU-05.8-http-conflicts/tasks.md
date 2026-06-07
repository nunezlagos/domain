# Tasks: HU-05.8-http-conflicts

## Backend

- [ ] **B1: Definir ConflictRepo interface y tipos**
      - ConflictGroup, Judgment, Comparison, ConflictStats, ScanResult, DeferredItem, ReplayResult structs
      - ConflictRepo interface con List, Judge, Compare, GetByID, Stats, Scan, ListDeferred, ReplayDeferred

- [ ] **B2: Handler GET /conflicts**
      - Llamar `repo.List(ctx)`
      - Responder 200 con []ConflictGroup

- [ ] **B3: Handler POST /conflicts/judge**
      - Parsear body: `{ observation_ids: [...], marked_by_model: "..." }`
      - Llamar `repo.Judge(ctx, ids, model)`
      - Responder 200 con []Judgment

- [ ] **B4: Handler POST /conflicts/compare**
      - Parsear body: `{ id_a: int, id_b: int }`
      - Llamar `repo.Compare(ctx, idA, idB)`
      - Si alguna no existe → 404
      - Responder 200 con Comparison

- [ ] **B5: Handler GET /conflicts/{id}**
      - Parsear id como int
      - Llamar `repo.GetByID(ctx, id)`
      - NotFound → 404
      - Success → 200

- [ ] **B6: Handler GET /conflicts/stats**
      - Llamar `repo.Stats(ctx)`
      - Responder 200 con ConflictStats

- [ ] **B7: Handler POST /conflicts/scan**
      - Parsear body: `{ project?: string }`
      - Llamar `repo.Scan(ctx, project)`
      - Responder 200 con ScanResult

- [ ] **B8: Handler GET /conflicts/deferred**
      - Llamar `repo.ListDeferred(ctx)`
      - Responder 200 con []DeferredItem

- [ ] **B9: Handler POST /conflicts/deferred/replay**
      - Llamar `repo.ReplayDeferred(ctx)`
      - Responder 200 con ReplayResult

- [ ] **B10: Implementar store layer (thin wrappers que delegan en REQ-10)**
      - Cada método en store/conflict.go llama al módulo correspondiente de conflict/
      - Si el módulo no existe aún, retornar array vacío o error temporal

- [ ] **B11: RegisterConflictRoutes**

## Frontend

- [ ] N/A — HU puramente backend

## Tests

- [ ] **T1: TestListConflicts — GET /conflicts array vacío**
      ```go
      func TestListConflicts(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          resp, _ := http.Get(srv.URL + "/conflicts")
          assert.Equal(t, 200, resp.StatusCode)
          var groups []ConflictGroup
          json.NewDecoder(resp.Body).Decode(&groups)
          assert.Empty(t, groups) // no conflicts initially
      }
      ```

- [ ] **T2: TestConflictStats — GET /conflicts/stats**
      ```go
      func TestConflictStats(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          resp, _ := http.Get(srv.URL + "/conflicts/stats")
          assert.Equal(t, 200, resp.StatusCode)
          var stats ConflictStats
          json.NewDecoder(resp.Body).Decode(&stats)
          assert.NotNil(t, stats)
      }
      ```

- [ ] **T3: TestConflictCompare — POST /conflicts/compare**
      ```go
      func TestConflictCompare(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          oid1 := createObs(t, srv.URL, sid, "same content")
          oid2 := createObs(t, srv.URL, sid, "same content")
          body := fmt.Sprintf(`{"id_a":%d,"id_b":%d}`, oid1, oid2)
          resp, _ := http.Post(srv.URL+"/conflicts/compare", "application/json",
              strings.NewReader(body))
          assert.Equal(t, 200, resp.StatusCode)
          var comp Comparison
          json.NewDecoder(resp.Body).Decode(&comp)
          assert.Greater(t, comp.SimilarityScore, 0.0)
      }
      ```

- [ ] **T4: TestConflictGetByID — GET /conflicts/{id}**
      ```go
      func TestConflictGetByID(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          // Create a conflict first via scan, then fetch it
          // For now, test that the route exists
          resp, _ := http.Get(srv.URL + "/conflicts/1")
          assert.Equal(t, 200, resp.StatusCode)
      }
      ```

- [ ] **T5: TestConflictScan — POST /conflicts/scan**
      ```go
      func TestConflictScan(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          resp, _ := http.Post(srv.URL+"/conflicts/scan", "application/json",
              strings.NewReader(`{}`))
          assert.Equal(t, 200, resp.StatusCode)
          var sr ScanResult
          json.NewDecoder(resp.Body).Decode(&sr)
          assert.NotNil(t, sr)
      }
      ```

- [ ] **T6: TestDeferredList — GET /conflicts/deferred**
      ```go
      func TestDeferredList(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          resp, _ := http.Get(srv.URL + "/conflicts/deferred")
          assert.Equal(t, 200, resp.StatusCode)
          var items []DeferredItem
          json.NewDecoder(resp.Body).Decode(&items)
          assert.Empty(t, items)
      }
      ```

- [ ] **T7: TestDeferredReplay — POST /conflicts/deferred/replay**
      ```go
      func TestDeferredReplay(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          resp, _ := http.Post(srv.URL+"/conflicts/deferred/replay", "application/json", nil)
          assert.Equal(t, 200, resp.StatusCode)
          var rr ReplayResult
          json.NewDecoder(resp.Body).Decode(&rr)
          assert.NotNil(t, rr)
      }
      ```

- [ ] **T8: TestConflictJudge — POST /conflicts/judge**
      ```go
      func TestConflictJudge(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          oid1 := createObs(t, srv.URL, sid, "judge me")
          oid2 := createObs(t, srv.URL, sid, "judge me too")
          body := fmt.Sprintf(`{"observation_ids":[%d,%d],"marked_by_model":"test"}`, oid1, oid2)
          resp, _ := http.Post(srv.URL+"/conflicts/judge", "application/json",
              strings.NewReader(body))
          assert.Equal(t, 200, resp.StatusCode)
      }
      ```

- [ ] **T9: Sabotaje — Scan sin límite**
      1. Comentar limit en Scan
      2. Insertar 2000 observations
      3. Scan puede ser muy lento o timeout
      4. Restaurar límite
      5. Test de performance pasa

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/api/... -v` — suite completa verde
- [ ] Probar manualmente con curl
- [ ] Commit: `feat: HTTP conflict detection and resolution endpoints`
