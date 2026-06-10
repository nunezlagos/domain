# Tasks: issue-05.4-http-prompts

## Backend

- [ ] **B1: Definir PromptRepo interface y tipos**
      - Prompt struct con tags json
      - PromptRepo interface con Save, Recent, Search, Delete

- [ ] **B2: Handler POST /prompts**
      - Parsear body: `{ session_id, content, project }`
      - Validar: session_id requerido, content requerido → 400
      - Llamar `repo.Save(ctx, prompt)`
      - Responder 201

- [ ] **B3: Handler GET /prompts/recent**
      - Leer `?limit=` (default 20, max 100)
      - Llamar `repo.Recent(ctx, limit)`
      - Responder 200 con []Prompt

- [ ] **B4: Handler GET /prompts/search**
      - Leer query params: `q` (required), `project` (optional)
      - Validar q no vacío → 400
      - Llamar `repo.Search(ctx, q, project)`
      - Responder 200 con []Prompt

- [ ] **B5: Handler DELETE /prompts/{id}**
      - Parsear id como int
      - Llamar `repo.Delete(ctx, id)`
      - NotFound → 404
      - Success → 204

- [ ] **B6: Implementar PromptRepo en store**
      - Save: INSERT INTO user_prompts
      - Recent: SELECT con ORDER BY created_at DESC LIMIT
      - Search: FTS5 MATCH + JOIN + optional project filter
      - Delete: DELETE FROM user_prompts WHERE id = ?; verificar rows affected

- [ ] **B7: RegisterPromptRoutes**

## Frontend

- [ ] N/A — HU puramente backend

## Tests

- [ ] **T1: TestSavePrompt — POST 201**
      ```go
      func TestSavePrompt(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          body := fmt.Sprintf(`{"session_id":"%s","content":"test prompt","project":"myapp"}`, sid)
          resp, _ := http.Post(srv.URL+"/prompts", "application/json", strings.NewReader(body))
          assert.Equal(t, 201, resp.StatusCode)
          var p Prompt
          json.NewDecoder(resp.Body).Decode(&p)
          assert.NotZero(t, p.ID)
          assert.Equal(t, "test prompt", p.Content)
      }
      ```

- [ ] **T2: TestSavePromptMissingContent — 400**
      ```go
      func TestSavePromptMissingContent(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          resp, _ := http.Post(srv.URL+"/prompts", "application/json",
              strings.NewReader(`{"session_id":"s1"}`))
          assert.Equal(t, 400, resp.StatusCode)
      }
      ```

- [ ] **T3: TestSavePromptMissingSessionID — 400**
      ```go
      func TestSavePromptMissingSessionID(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          resp, _ := http.Post(srv.URL+"/prompts", "application/json",
              strings.NewReader(`{"content":"test"}`))
          assert.Equal(t, 400, resp.StatusCode)
      }
      ```

- [ ] **T4: TestRecentPrompts — GET array DESC**
      ```go
      func TestRecentPrompts(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          savePrompt(t, srv.URL, sid, "first")
          savePrompt(t, srv.URL, sid, "second")
          resp, _ := http.Get(srv.URL + "/prompts/recent")
          assert.Equal(t, 200, resp.StatusCode)
          var prompts []Prompt
          json.NewDecoder(resp.Body).Decode(&prompts)
          assert.GreaterOrEqual(t, len(prompts), 2)
      }
      ```

- [ ] **T5: TestSearchPrompts — GET /prompts/search**
      ```go
      func TestSearchPrompts(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          savePrompt(t, srv.URL, sid, "architecture decision")
          savePrompt(t, srv.URL, sid, "random note")
          resp, _ := http.Get(srv.URL + "/prompts/search?q=architecture")
          assert.Equal(t, 200, resp.StatusCode)
          var prompts []Prompt
          json.NewDecoder(resp.Body).Decode(&prompts)
          assert.GreaterOrEqual(t, len(prompts), 1)
          assert.Contains(t, prompts[0].Content, "architecture")
      }
      ```

- [ ] **T6: TestSearchPromptsEmptyQuery — 400**
      ```go
      func TestSearchPromptsEmptyQuery(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          resp, _ := http.Get(srv.URL + "/prompts/search")
          assert.Equal(t, 400, resp.StatusCode)
      }
      ```

- [ ] **T7: TestDeletePrompt — 204**
      ```go
      func TestDeletePrompt(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          sid := createTestSessionID(t, srv.URL)
          p := savePrompt(t, srv.URL, sid, "delete me")
          req, _ := http.NewRequest("DELETE", srv.URL+"/prompts/"+strconv.Itoa(p.ID), nil)
          resp, _ := http.DefaultClient.Do(req)
          assert.Equal(t, 204, resp.StatusCode)
      }
      ```

- [ ] **T8: TestDeletePromptNotFound — 404**
      ```go
      func TestDeletePromptNotFound(t *testing.T) {
          srv := newTestServer(t)
          defer srv.Close()
          req, _ := http.NewRequest("DELETE", srv.URL+"/prompts/99999", nil)
          resp, _ := http.DefaultClient.Do(req)
          assert.Equal(t, 404, resp.StatusCode)
      }
      ```

- [ ] **T9: Sabotaje — delete sin check de existencia**
      1. En Delete handler, no verificar rows affected
      2. DELETE /prompts/99999 → 204 (esperaba 404)
      3. Restaurar con check de rows affected == 0 → 404
      4. Test pasa

- [ ] **T10: Helper savePrompt**
      ```go
      func savePrompt(t *testing.T, baseURL, sessionID, content string) Prompt {
          body := fmt.Sprintf(`{"session_id":"%s","content":"%s"}`, sessionID, content)
          resp, _ := http.Post(baseURL+"/prompts", "application/json", strings.NewReader(body))
          var p Prompt
          json.NewDecoder(resp.Body).Decode(&p)
          return p
      }
      ```

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/api/... -v` — suite completa verde
- [ ] Probar manualmente con curl
- [ ] Commit: `feat: HTTP prompts CRUD with FTS5 search`
