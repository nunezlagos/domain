# Tasks: HU-05.2-http-observations

## Backend

- [ ] **B1: Definir ObservationRepo interface y tipos**
      - Observation struct con tags json
      - ObservationFilter struct (limit, project, type, scope)
      - ConflictCandidate struct
      - ObservationRepo interface con Create, CreatePassive, Recent, GetByID, Update, SoftDelete, HardDelete

- [ ] **B2: Handler POST /observations**
      - Parsear `CreateObservationRequest` del body
      - Validar: session_id requerido, content requerido
      - Llamar `repo.Create` que retorna (Observation, *ConflictCandidate, error)
      - Si conflict candidate existe, incluirlo en response
      - Responder 201

- [ ] **B3: Implementar conflict detection en Create**
      - Calcular normalized_hash con `dedup.Normalize(content)`
      - Buscar en DB: `SELECT id, title FROM observations WHERE normalized_hash = ? AND deleted_at IS NULL LIMIT 1`
      - Si existe, retornar ConflictCandidate con similarity_score = 1.0

- [ ] **B4: Handler POST /observations/passive**
      - Parsear body: `{ session_id, content, source? }`
      - Llamar `repo.CreatePassive` (sin conflict detection)
      - Responder 201

- [ ] **B5: Handler GET /observations/recent**
      - Leer query params: limit (default 20, max 100), project, type, scope
      - Construir query SQL dinámica con filtros opcionales
      - `WHERE deleted_at IS NULL` + ORDER BY created_at DESC LIMIT ?
      - Responder 200 con []Observation

- [ ] **B6: Handler GET /observations/{id}**
      - Parsear id como int
      - Llamar `repo.GetByID(ctx, id)`
      - NotFound → 404
      - Success → 200

- [ ] **B7: Handler PATCH /observations/{id}**
      - Parsear id como int
      - Leer body como `map[string]any` (json.RawMessage)
      - Llamar `repo.GetByID` → 404 si no existe
      - Aplicar merge con `applyPatch`
      - Llamar `repo.Update(ctx, merged)`
      - Responder 200

- [ ] **B8: Handler DELETE /observations/{id}**
      - Leer `?hard=true` de query
      - Hard: llamar `repo.HardDelete` (requiere auth, HU-05.9 marcará esto)
      - Soft: llamar `repo.SoftDelete`
      - NotFound → 404
      - Success → 204

- [ ] **B9: RegisterObservationRoutes**
      ```go
      func RegisterObservationRoutes(mux *http.ServeMux, repo ObservationRepo) {
          mux.HandleFunc("POST /observations", handleCreate(repo))
          mux.HandleFunc("POST /observations/passive", handleCreatePassive(repo))
          mux.HandleFunc("GET /observations/recent", handleRecent(repo))
          mux.HandleFunc("GET /observations/{id}", handleGetByID(repo))
          mux.HandleFunc("PATCH /observations/{id}", handleUpdate(repo))
          mux.HandleFunc("DELETE /observations/{id}", handleDelete(repo))
      }
      ```

## Frontend

- [ ] N/A — HU puramente backend

## Tests

- [ ] **T1: TestCreateObservation — POST 201**
      ```go
      func TestCreateObservation(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          body := fmt.Sprintf(`{"session_id":"%s","title":"Test","content":"Hello"}`, sid)
          resp, _ := http.Post(srv.URL+"/observations", "application/json", strings.NewReader(body))
          assert.Equal(t, 201, resp.StatusCode)
          var obs Observation
          json.NewDecoder(resp.Body).Decode(&obs)
          assert.NotZero(t, obs.ID)
          assert.Equal(t, "Test", obs.Title)
      }
      ```

- [ ] **T2: TestCreateObservationConflictCandidate — detecta duplicado**
      ```go
      func TestCreateObservationConflictCandidate(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          body := fmt.Sprintf(`{"session_id":"%s","content":"exact duplicate content"}`, sid)
          http.Post(srv.URL+"/observations", "application/json", strings.NewReader(body))
          resp, _ := http.Post(srv.URL+"/observations", "application/json", strings.NewReader(body))
          var result struct {
              Observation
              ConflictCandidate *ConflictCandidate `json:"conflict_candidate"`
          }
          json.NewDecoder(resp.Body).Decode(&result)
          assert.NotNil(t, result.ConflictCandidate)
          assert.Equal(t, 1.0, result.ConflictCandidate.SimilarityScore)
      }
      ```

- [ ] **T3: TestCreateObservationMissingSessionID — 400**
      ```go
      func TestCreateObservationMissingSessionID(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          resp, _ := http.Post(srv.URL+"/observations", "application/json",
              strings.NewReader(`{"content":"orphan"}`))
          assert.Equal(t, 400, resp.StatusCode)
      }
      ```

- [ ] **T4: TestRecentObservations — GET retorna array DESC**
      ```go
      func TestRecentObservations(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          createObs(t, srv.URL, sid, "first")
          createObs(t, srv.URL, sid, "second")
          resp, _ := http.Get(srv.URL + "/observations/recent")
          assert.Equal(t, 200, resp.StatusCode)
          var obs []Observation
          json.NewDecoder(resp.Body).Decode(&obs)
          assert.GreaterOrEqual(t, len(obs), 2)
      }
      ```

- [ ] **T5: TestRecentObservationsFiltered — filtros aplican**
      ```go
      func TestRecentObservationsFiltered(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          createObsWithType(t, srv.URL, sid, "decision")
          createObsWithType(t, srv.URL, sid, "general")
          resp, _ := http.Get(srv.URL + "/observations/recent?type=decision")
          var obs []Observation
          json.NewDecoder(resp.Body).Decode(&obs)
          for _, o := range obs {
              assert.Equal(t, "decision", o.Type)
          }
      }
      ```

- [ ] **T6: TestGetObservation — 200**
      ```go
      func TestGetObservation(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          oid := createObs(t, srv.URL, sid, "getme")
          resp, _ := http.Get(srv.URL + "/observations/" + strconv.Itoa(oid))
          assert.Equal(t, 200, resp.StatusCode)
      }
      ```

- [ ] **T7: TestGetObservationNotFound — 404**
      ```go
      func TestGetObservationNotFound(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          resp, _ := http.Get(srv.URL + "/observations/99999")
          assert.Equal(t, 404, resp.StatusCode)
      }
      ```

- [ ] **T8: TestUpdateObservation — PATCH 200**
      ```go
      func TestUpdateObservation(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          oid := createObs(t, srv.URL, sid, "original")
          body := fmt.Sprintf(`{"title":"updated","content":"new content","revision_count":2}`)
          req, _ := http.NewRequest("PATCH", srv.URL+"/observations/"+strconv.Itoa(oid),
              strings.NewReader(body))
          req.Header.Set("Content-Type", "application/json")
          resp, _ := http.DefaultClient.Do(req)
          assert.Equal(t, 200, resp.StatusCode)
          var obs Observation
          json.NewDecoder(resp.Body).Decode(&obs)
          assert.Equal(t, "updated", obs.Title)
          assert.Equal(t, "new content", obs.Content)
      }
      ```

- [ ] **T9: TestSoftDelete — DELETE 204 luego GET 404**
      ```go
      func TestSoftDelete(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          oid := createObs(t, srv.URL, sid, "todelete")
          req, _ := http.NewRequest("DELETE", srv.URL+"/observations/"+strconv.Itoa(oid), nil)
          resp, _ := http.DefaultClient.Do(req)
          assert.Equal(t, 204, resp.StatusCode)
          resp2, _ := http.Get(srv.URL + "/observations/" + strconv.Itoa(oid))
          assert.Equal(t, 404, resp2.StatusCode)
      }
      ```

- [ ] **T10: TestHardDelete — DELETE?hard=true**
      ```go
      func TestHardDelete(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          oid := createObs(t, srv.URL, sid, "harddelete")
          req, _ := http.NewRequest("DELETE", srv.URL+"/observations/"+strconv.Itoa(oid)+"?hard=true", nil)
          resp, _ := http.DefaultClient.Do(req)
          assert.Equal(t, 204, resp.StatusCode)
      }
      ```

- [ ] **T11: TestPassiveCapture — POST /observations/passive 201**
      ```go
      func TestPassiveCapture(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          body := fmt.Sprintf(`{"session_id":"%s","content":"passive note","source":"tool"}`, sid)
          resp, _ := http.Post(srv.URL+"/observations/passive", "application/json", strings.NewReader(body))
          assert.Equal(t, 201, resp.StatusCode)
          var obs Observation
          json.NewDecoder(resp.Body).Decode(&obs)
          assert.NotZero(t, obs.ID)
      }
      ```

- [ ] **T12: Sabotaje — eliminar WHERE deleted_at IS NULL**
      1. En handler GET /observations/{id}, sacar `AND deleted_at IS NULL`
      2. Ejecutar TestSoftDelete → FALLA (GET retorna 200 en vez de 404)
      3. Restaurar
      4. Test pasa

- [ ] **T13: Helpers de test**
      ```go
      func createTestSessionID(t *testing.T, baseURL string) string {
          resp, _ := http.Post(baseURL+"/sessions", "application/json",
              strings.NewReader(`{"directory":"/tmp"}`))
          var s Session
          json.NewDecoder(resp.Body).Decode(&s)
          return s.ID
      }

      func createObs(t *testing.T, baseURL, sessionID, content string) int {
          body := fmt.Sprintf(`{"session_id":"%s","content":"%s"}`, sessionID, content)
          resp, _ := http.Post(baseURL+"/observations", "application/json", strings.NewReader(body))
          var obs Observation
          json.NewDecoder(resp.Body).Decode(&obs)
          return obs.ID
      }
      ```

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/api/... -v` — suite completa verde
- [ ] Probar manualmente con curl
- [ ] Commit: `feat: HTTP observations CRUD with conflict detection and passive capture`
