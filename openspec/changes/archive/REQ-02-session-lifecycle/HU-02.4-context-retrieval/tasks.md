# Tasks: HU-02.4-context-retrieval

## Backend

- [ ] **B1: Crear `internal/store/context.go` con tipos base**
      - Tipo `Scope` con constantes `ScopeProject`, `ScopePersonal`, `ScopeGlobal`
      - Struct `ContextQuery` con Project, Scope, Limit
      - Struct `ContextResult` con Sessions, Observations, Prompts slices
      - Constante `defaultLimit = 10`
      - Constante `maxLimit = 100`

- [ ] **B2: Implementar `SessionStore.GetRecentSessions(ctx, project, limit)`**
      - Query: `SELECT id, project, directory, started_at, ended_at, summary, status FROM sessions WHERE project = ? ORDER BY started_at DESC LIMIT ?`
      - Si project vacío, no filtrar por proyecto (scope=global)
      - Si project no vacío, filtrar
      - Mapear rows a `[]Session`

- [ ] **B3: Implementar `ObservationStore.GetRecentObservations(ctx, project, scope, limit)`**
      - Query builder dinámico:
        - `WHERE deleted_at IS NULL`
        - `AND scope = ?` si scope != global
        - `AND project = ?` si project != "" y scope != "personal"
        - `ORDER BY created_at DESC LIMIT ?`
      - Si scope=personal, NO filtrar por project (cross-project)
      - Si scope=global, no filtrar ni por scope ni por project

- [ ] **B4: Implementar `PromptStore.GetRecentPrompts(ctx, project, limit)`**
      - `SELECT id, session_id, content, project, created_at FROM user_prompts WHERE project = ? ORDER BY created_at DESC LIMIT ?`
      - Si project vacío, omitir WHERE project

- [ ] **B5: Implementar `GetContext(ctx, query ContextQuery) (*ContextResult, error)`**
      - Normalizar limit (default 10, max 100)
      - Normalizar scope: si vacío, default ScopeProject
      - Llamar a los 3 stores secuencialmente
      - Retornar ContextResult con los resultados

- [ ] **B6: Implementar `FormatContext(result *ContextResult) string`**
      - Output en markdown con secciones:
        - `# Session Context`
        - `## Recent Sessions` — tabla con id, project, started_at, status
        - `## Recent Observations` — bullets con type, scope, content truncado
        - `## Recent Prompts` — bullets con timestamp y content truncado
      - Si una sección está vacía: `_No recent sessions._` etc.
      - Truncar content a 200 caracteres con `...`

## Frontend

- [ ] N/A — HU puramente backend (el output es texto para consumo del agente)

## Tests

- [ ] **T1: TestGetRecentSessions — filtrar por proyecto**
      ```go
      func TestGetRecentSessions(t *testing.T) {
          db := setupTestDB(t)
          sessionStore := NewSessionStore(db)
          sessionStore.Start(ctx, "s1", "Domain", "/tmp")
          sessionStore.Start(ctx, "s2", "Domain", "/tmp")
          sessionStore.Start(ctx, "s3", "otro", "/tmp")
          sessions, err := sessionStore.GetRecentSessions(ctx, "Domain", 10)
          require.NoError(t, err)
          assert.Equal(t, 2, len(sessions))
      }
      ```

- [ ] **T2: TestGetRecentObservationsByScope — filtrar por scope**
      ```go
      func TestGetRecentObservationsByScope(t *testing.T) {
          db := setupTestDB(t)
          obsStore := NewObservationStore(db)
          // crear observaciones con diferentes scopes
          addObs(t, obsStore, "s1", "proj-a", "public", "project")
          addObs(t, obsStore, "s1", "proj-a", "secreto", "personal")
          obs, err := obsStore.GetRecentObservations(ctx, "proj-a", ScopePersonal, 10)
          require.NoError(t, err)
          assert.Equal(t, 1, len(obs))
          assert.Equal(t, "personal", obs[0].Scope)
      }
      ```

- [ ] **T3: TestGetRecentObservationsCrossProject — scope personal cruza proyectos**
      ```go
      func TestGetRecentObservationsCrossProject(t *testing.T) {
          db := setupTestDB(t)
          obsStore := NewObservationStore(db)
          addObs(t, obsStore, "s1", "proj-a", "personal-a", "personal")
          addObs(t, obsStore, "s2", "proj-b", "personal-b", "personal")
          obs, err := obsStore.GetRecentObservations(ctx, "", ScopePersonal, 10)
          require.NoError(t, err)
          assert.Equal(t, 2, len(obs))
      }
      ```

- [ ] **T4: TestGetContextDefaultLimit — sin limit usa default 10**
      ```go
      func TestGetContextDefaultLimit(t *testing.T) {
          db := setupTestDB(t)
          svc := NewContextService(db)
          result, err := svc.GetContext(ctx, ContextQuery{Project: "Domain", Scope: ScopeProject})
          require.NoError(t, err)
          assert.NotNil(t, result)
          // sin datos, slices vacíos pero no error
      }
      ```

- [ ] **T5: TestGetContextEmptyProject — proyecto sin datos**
      ```go
      func TestGetContextEmptyProject(t *testing.T) {
          db := setupTestDB(t)
          svc := NewContextService(db)
          result, err := svc.GetContext(ctx, ContextQuery{Project: "vacio", Scope: ScopeProject, Limit: 5})
          require.NoError(t, err)
          assert.Empty(t, result.Sessions)
          assert.Empty(t, result.Observations)
          assert.Empty(t, result.Prompts)
      }
      ```

- [ ] **T6: TestFormatContext — formato markdown correcto**
      ```go
      func TestFormatContext(t *testing.T) {
          result := &ContextResult{
              Sessions: []Session{{ID: "s1", Project: "p1", Status: "active"}},
          }
          output := FormatContext(result)
          assert.Contains(t, output, "# Session Context")
          assert.Contains(t, output, "## Recent Sessions")
          assert.Contains(t, output, "s1")
          assert.Contains(t, output, "_No recent observations._")
      }
      ```

- [ ] **T7: TestGetRecentObservationsScopeGlobal — sin filtros**
      ```go
      func TestGetRecentObservationsScopeGlobal(t *testing.T) {
          db := setupTestDB(t)
          obsStore := NewObservationStore(db)
          addObs(t, obsStore, "s1", "proj-a", "content-a", "project")
          addObs(t, obsStore, "s2", "proj-b", "content-b", "personal")
          obs, err := obsStore.GetRecentObservations(ctx, "", ScopeGlobal, 10)
          require.NoError(t, err)
          assert.Equal(t, 2, len(obs))
      }
      ```

- [ ] **T8: TestGetRecentSessionsOrder — orden DESC por started_at**
      ```go
      func TestGetRecentSessionsOrder(t *testing.T) {
          db := setupTestDB(t)
          sessionStore := NewSessionStore(db)
          sessionStore.Start(ctx, "s1", "p1", "/d1") // más vieja
          time.Sleep(10 * time.Millisecond)
          sessionStore.Start(ctx, "s2", "p1", "/d1") // más nueva
          sessions, _ := sessionStore.GetRecentSessions(ctx, "p1", 10)
          require.Len(t, sessions, 2)
          assert.Equal(t, "s2", sessions[0].ID)
          assert.Equal(t, "s1", sessions[1].ID)
      }
      ```

- [ ] **T9: TestGetContextLimitCap — limit mayor a max se capea**
      ```go
      func TestGetContextLimitCap(t *testing.T) {
          db := setupTestDB(t)
          svc := NewContextService(db)
          for i := 0; i < 150; i++ {
              sid := fmt.Sprintf("s%d", i)
              NewSessionStore(db).Start(ctx, sid, "p1", "/d1")
          }
          result, err := svc.GetContext(ctx, ContextQuery{Project: "p1", Scope: ScopeProject, Limit: 1000})
          require.NoError(t, err)
          assert.LessOrEqual(t, len(result.Sessions), 100)
      }
      ```

- [ ] **T10: Sabotaje — SQL injection en project**
      1. Pasar `project = "' OR 1=1 --"` como filtro
      2. Si hay inyección, retornaría sesiones de otros proyectos
      3. Verificar que solo retorna vacío (proyecto literal no existe)
      4. Si el test pasa (solo project literal), la sanitización con `?` funciona
      5. Si el test falla (retorna datos), hay vulnerability
      6. Documentar el sabotaje

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/store/... -v` — suite completa verde
- [ ] Commit: `feat: add context retrieval with scope filtering for agents`
