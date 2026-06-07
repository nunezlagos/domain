# Tasks: HU-05.3-http-search-context

## Backend

- [ ] **B1: Definir SearchRepo interface y tipos**
      - SearchResult, SearchFilter, TimelineResponse structs
      - SearchRepo interface con Search, Timeline, Context methods

- [ ] **B2: Implementar FTS5 sanitizer**
      - `sanitizeFTS5(q string) string`: remover chars especiales, agregar `*` suffix para prefix matching
      - Si query queda vacía tras sanitizar, retornar error

- [ ] **B3: Handler GET /search**
      - Leer query params: q (required), type, project, scope, limit
      - Validar q no vacío → 400 si vacío
      - Sanitizar q para FTS5
      - Construir SearchFilter con params
      - Llamar `repo.Search(ctx, q, filter)`
      - Responder 200 con []SearchResult

- [ ] **B4: Implementar Search en store**
      - Query FTS5 con MATCH + JOIN observations
      - WHERE dinámico para type/project/scope
      - ORDER BY rank DESC
      - LIMIT default 20, max 100
      - Excluir deleted_at IS NOT NULL

- [ ] **B5: Handler GET /timeline**
      - Leer query params: observation_id (required int), before (int, default 0), after (int, default 0)
      - Llamar `repo.Timeline(ctx, obsID, before, after)`
      - Si observation no existe → 404
      - Responder 200 con TimelineResponse

- [ ] **B6: Implementar Timeline en store**
      - GetByID para center (404 si no existe)
      - Query before: `SELECT ... WHERE id < ? ORDER BY id DESC LIMIT ?`
      - Query after: `SELECT ... WHERE id > ? ORDER BY id ASC LIMIT ?`
      - Ambos con `deleted_at IS NULL`

- [ ] **B7: Handler GET /context**
      - Leer query params: project (required), scope (optional)
      - Validar project no vacío → 400
      - Llamar `repo.Context(ctx, project, scope)`
      - Responder 200 con `{"project":"...","observations":[...],"count":N}`

- [ ] **B8: Implementar Context en store**
      - `SELECT ... FROM observations WHERE project = ? AND deleted_at IS NULL`
      - Si scope no vacío, agregar `AND scope = ?`

- [ ] **B9: RegisterSearchRoutes**

## Frontend

- [ ] N/A — HU puramente backend

## Tests

- [ ] **T1: TestSearch — GET /search con resultados**
      ```go
      func TestSearch(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          createObs(t, srv.URL, sid, "hello world")
          createObs(t, srv.URL, sid, "goodbye world")
          resp, _ := http.Get(srv.URL + "/search?q=hello")
          assert.Equal(t, 200, resp.StatusCode)
          var results []SearchResult
          json.NewDecoder(resp.Body).Decode(&results)
          assert.GreaterOrEqual(t, len(results), 1)
          assert.Contains(t, results[0].Content, "hello")
      }
      ```

- [ ] **T2: TestSearchEmptyQuery — 400**
      ```go
      func TestSearchEmptyQuery(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          resp, _ := http.Get(srv.URL + "/search")
          assert.Equal(t, 400, resp.StatusCode)
          resp2, _ := http.Get(srv.URL + "/search?q=")
          assert.Equal(t, 400, resp2.StatusCode)
      }
      ```

- [ ] **T3: TestSearchWithFilters**
      ```go
      func TestSearchWithFilters(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          // Create with type=decision
          body := fmt.Sprintf(`{"session_id":"%s","content":"arch decision","type":"decision"}`, sid)
          http.Post(srv.URL+"/observations", "application/json", strings.NewReader(body))
          resp, _ := http.Get(srv.URL + "/search?q=arch&type=decision")
          var results []SearchResult
          json.NewDecoder(resp.Body).Decode(&results)
          for _, r := range results {
              assert.Equal(t, "decision", r.Type)
          }
      }
      ```

- [ ] **T4: TestSearchLimit**
      ```go
      func TestSearchLimit(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          for i := 0; i < 10; i++ {
              createObs(t, srv.URL, sid, fmt.Sprintf("common word %d", i))
          }
          resp, _ := http.Get(srv.URL + "/search?q=common&limit=3")
          var results []SearchResult
          json.NewDecoder(resp.Body).Decode(&results)
          assert.LessOrEqual(t, len(results), 3)
      }
      ```

- [ ] **T5: TestTimeline — 200 con center + before + after**
      ```go
      func TestTimeline(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          oid := createObs(t, srv.URL, sid, "center")
          createObs(t, srv.URL, sid, "before1")
          createObs(t, srv.URL, sid, "before2")
          createObs(t, srv.URL, sid, "after1")
          resp, _ := http.Get(fmt.Sprintf("%s/timeline?observation_id=%d&before=2&after=2", srv.URL, oid))
          assert.Equal(t, 200, resp.StatusCode)
          var tl TimelineResponse
          json.NewDecoder(resp.Body).Decode(&tl)
          assert.Equal(t, oid, tl.Center.ID)
      }
      ```

- [ ] **T6: TestTimelineNotFound — 404**
      ```go
      func TestTimelineNotFound(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          resp, _ := http.Get(srv.URL + "/timeline?observation_id=99999&before=2")
          assert.Equal(t, 404, resp.StatusCode)
      }
      ```

- [ ] **T7: TestContext — GET /context con project**
      ```go
      func TestContext(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          body := fmt.Sprintf(`{"session_id":"%s","content":"ctx1","project":"myapp","scope":"project"}`, sid)
          http.Post(srv.URL+"/observations", "application/json", strings.NewReader(body))
          resp, _ := http.Get(srv.URL + "/context?project=myapp&scope=project")
          assert.Equal(t, 200, resp.StatusCode)
          var cr struct {
              Project      string        `json:"project"`
              Observations []Observation `json:"observations"`
              Count        int           `json:"count"`
          }
          json.NewDecoder(resp.Body).Decode(&cr)
          assert.Equal(t, "myapp", cr.Project)
          assert.GreaterOrEqual(t, cr.Count, 1)
      }
      ```

- [ ] **T8: TestContextMissingProject — 400**
      ```go
      func TestContextMissingProject(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          resp, _ := http.Get(srv.URL + "/context")
          assert.Equal(t, 400, resp.StatusCode)
      }
      ```

- [ ] **T9: Sabotaje — sacar deleted_at filter de Search**
      1. Eliminar `AND o.deleted_at IS NULL` de search query
      2. Soft-delete una observation, buscarla → aparece en resultados (FALLA)
      3. Restaurar → pasa

- [ ] **T10: FTS5 sanitizer test**
      ```go
      func TestSanitizeFTS5(t *testing.T) {
          assert.Equal(t, "hello* world*", sanitizeFTS5("hello world"))
          assert.Equal(t, "", sanitizeFTS5("***"))
          assert.Equal(t, "test*", sanitizeFTS5(`test"quote`))
      }
      ```

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/api/... -v` — suite completa verde
- [ ] Probar manualmente con curl
- [ ] Commit: `feat: HTTP search, timeline, and context retrieval endpoints`
