# Tasks: HU-01.4-deduplication

## Backend

- [ ] **B1: Definir interfaz Store y SaveResult**
      - `internal/store/store.go` — interfaz `Store` con métodos CRUD
      - `SaveResult` struct con `Observation`, `Deduplicated bool`, `OriginalID int64`

- [ ] **B2: Implementar NormalizeContent(content string) string**
      - Trim whitespace
      - Regex `[\s]+` → espacio simple
      - `strings.ToLower`
      - Tests en `normalize_test.go`

- [ ] **B3: Implementar ComputeHash(project, scope, typ, title, normalizedContent string) string**
      - Concatenar con pipe: `"project|scope|type|title|normalizedContent"`
      - SHA-256 → hex string (64 chars)
      - Tests en `hash_test.go`

- [ ] **B4: Agregar campo DedupWindow a Config**
      - `internal/config/config.go`:
        ```go
        type Config struct {
            DedupWindow time.Duration // 0 = disabled; default 60s
        }
        ```
      - Default: `DedupWindow: 60 * time.Second`

- [ ] **B5: Implementar DeduplicatingStore wrapper**
      - `internal/store/dedup.go`
      - Struct con `base Store` y `config Config`
      - Método `SaveObservation(ctx, obs)` que:
        1. Si `config.DedupWindow == 0` → delega directo
        2. Normaliza content, computa hash
        3. Consulta: `SELECT id, duplicate_count FROM observations WHERE normalized_hash = ? AND last_seen_at > datetime('now', ?) AND deleted_at IS NULL ORDER BY last_seen_at DESC LIMIT 1`
        4. Si existe → `UPDATE observations SET duplicate_count = duplicate_count + 1, last_seen_at = datetime('now') WHERE id = ?`
        5. Retorna `SaveResult{OriginalID: existingID, Deduplicated: true}`
        6. Si no existe → asigna `obs.NormalizedHash = hash`, delega a `base.SaveObservation`

- [ ] **B6: Integrar DeduplicatingStore en el constructor de Store**
      - `NewStore(db, config)` retorna `DeduplicatingStore{base: baseStore, config: config}`

- [ ] **B7: Asegurar que SaveObservation base persiste normalized_hash**
      - El CRUD base (HU-01.2) debe guardar `obs.NormalizedHash` en el INSERT
      - Si no existe, agregarlo como parte de esta HU

## Frontend

- [ ] N/A — HU puramente backend

## Tests

- [ ] **T1: TestNormalizeContent**
      ```go
      func TestNormalizeContent(t *testing.T) {
          tests := []struct{ input, expected string }{
              {"  Hello   World  ", "hello world"},
              {"A\n\nB\nC", "a b c"},
              {"\tFooo\t", "fooo"},
              {"  déjà vu  ", "déjà vu"}, // acentos preservados
          }
          for _, tt := range tests {
              assert.Equal(t, tt.expected, NormalizeContent(tt.input))
          }
      }
      ```

- [ ] **T2: TestComputeHash determinístico**
      ```go
      func TestComputeHash(t *testing.T) {
          h1 := ComputeHash("p", "s", "t", "title", "content")
          h2 := ComputeHash("p", "s", "t", "title", "content")
          assert.Equal(t, h1, h2)
          assert.Len(t, h1, 64) // SHA-256 hex
      }
      ```

- [ ] **T3: TestComputeHash diferentes inputs**
      ```go
      func TestComputeHashDifferent(t *testing.T) {
          h1 := ComputeHash("p1", "s", "t", "title", "content")
          h2 := ComputeHash("p2", "s", "t", "title", "content")
          assert.NotEqual(t, h1, h2)

          h3 := ComputeHash("p", "project", "t", "title", "content")
          h4 := ComputeHash("p", "personal", "t", "title", "content")
          assert.NotEqual(t, h3, h4)
      }
      ```

- [ ] **T4: TestDedupWithinWindow — duplicado dentro de ventana**
      ```go
      func TestDedupWithinWindow(t *testing.T) {
          db := setupTestDB(t)
          store := NewStore(db, Config{DedupWindow: 60 * time.Second})

          obs := Observation{SessionID: "s1", Project: "p", Scope: "project", Type: "general", Title: "T", Content: "hello world"}
          r1, err := store.SaveObservation(ctx, obs)
          require.NoError(t, err)
          require.False(t, r1.Deduplicated)

          r2, err := store.SaveObservation(ctx, obs)
          require.NoError(t, err)
          assert.True(t, r2.Deduplicated)
          assert.Equal(t, r1.Observation.ID, r2.OriginalID)

          var count int
          db.QueryRow("SELECT COUNT(*) FROM observations WHERE normalized_hash = ?", r1.Observation.NormalizedHash).Scan(&count)
          assert.Equal(t, 1, count) // solo 1 registro

          var dupCount int
          db.QueryRow("SELECT duplicate_count FROM observations WHERE id = ?", r1.Observation.ID).Scan(&dupCount)
          assert.Equal(t, 2, dupCount)
      }
      ```

- [ ] **T5: TestDedupOutsideWindow — misma hash fuera de ventana crea nuevo**
      ```go
      func TestDedupOutsideWindow(t *testing.T) {
          // Usar clock mock o ventana de 0s para simular "fuera de ventana"
          db := setupTestDB(t)
          store := NewStore(db, Config{DedupWindow: 0}) // dedup deshabilitado → no hay dedup

          obs := Observation{SessionID: "s1", Project: "p", Scope: "project", Type: "general", Title: "T", Content: "hello"}
          r1, _ := store.SaveObservation(ctx, obs)
          // Forzar last_seen_at viejo (directo en DB)

          store2 := NewStore(db, Config{DedupWindow: 60 * time.Second})
          r2, err := store2.SaveObservation(ctx, obs)
          require.NoError(t, err)
          assert.False(t, r2.Deduplicated) // nuevo registro
          assert.NotEqual(t, r1.Observation.ID, r2.Observation.ID)
      }
      ```

- [ ] **T6: TestDifferentScopeNoDedup**
      ```go
      func TestDifferentScopeNoDedup(t *testing.T) {
          db := setupTestDB(t)
          store := NewStore(db, Config{DedupWindow: 60 * time.Second})

          obs1 := Observation{SessionID: "s1", Project: "p", Scope: "project", Type: "general", Title: "T", Content: "hello"}
          obs2 := Observation{SessionID: "s1", Project: "p", Scope: "personal", Type: "general", Title: "T", Content: "hello"}

          r1, _ := store.SaveObservation(ctx, obs1)
          r2, _ := store.SaveObservation(ctx, obs2)

          assert.False(t, r2.Deduplicated)
          assert.NotEqual(t, r1.Observation.ID, r2.Observation.ID)
      }
      ```

- [ ] **T7: TestDifferentTypeNoDedup**
      ```go
      func TestDifferentTypeNoDedup(t *testing.T) {
          db := setupTestDB(t)
          store := NewStore(db, Config{DedupWindow: 60 * time.Second})

          obs1 := Observation{SessionID: "s1", Project: "p", Scope: "project", Type: "general", Title: "T", Content: "hello"}
          obs2 := Observation{SessionID: "s1", Project: "p", Scope: "project", Type: "decision", Title: "T", Content: "hello"}

          r1, _ := store.SaveObservation(ctx, obs1)
          r2, _ := store.SaveObservation(ctx, obs2)

          assert.False(t, r2.Deduplicated)
          assert.NotEqual(t, r1.Observation.ID, r2.Observation.ID)
      }
      ```

- [ ] **T8: TestDifferentProjectNoDedup**
      ```go
      func TestDifferentProjectNoDedup(t *testing.T) {
          db := setupTestDB(t)
          store := NewStore(db, Config{DedupWindow: 60 * time.Second})

          obs1 := Observation{SessionID: "s1", Project: "proj-a", Scope: "project", Type: "general", Title: "T", Content: "hello"}
          obs2 := Observation{SessionID: "s1", Project: "proj-b", Scope: "project", Type: "general", Title: "T", Content: "hello"}

          r1, _ := store.SaveObservation(ctx, obs1)
          r2, _ := store.SaveObservation(ctx, obs2)

          assert.False(t, r2.Deduplicated)
          assert.NotEqual(t, r1.Observation.ID, r2.Observation.ID)
      }
      ```

- [ ] **T9: TestDedupWindowZero — 0 deshabilita dedup completamente**
      ```go
      func TestDedupWindowZero(t *testing.T) {
          db := setupTestDB(t)
          store := NewStore(db, Config{DedupWindow: 0})

          obs := Observation{SessionID: "s1", Project: "p", Scope: "project", Type: "general", Title: "T", Content: "hello"}
          r1, _ := store.SaveObservation(ctx, obs)
          r2, _ := store.SaveObservation(ctx, obs)

          assert.False(t, r2.Deduplicated)
          assert.NotEqual(t, r1.Observation.ID, r2.Observation.ID) // dos registros
      }
      ```

- [ ] **T10: Sabotaje — deshabilitar query de dedup → test de duplicado cae**
      1. Comentar la consulta SQL de dedup en `DeduplicatingStore.SaveObservation` (siempre insertar)
      2. Ejecutar `TestDedupWithinWindow` → debe fallar (no detecta duplicado)
      3. Restaurar la consulta
      4. Ejecutar `TestDedupWithinWindow` nuevamente → debe pasar
      5. Documentar el sabotaje en comentario del test

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/store/... -v` — suite completa verde
- [ ] Verificar cobertura de todos los scenarios Gherkin en tests
- [ ] Commit: `feat: add exact deduplication with configurable rolling time window`

## Dependencias

- [ ] HU-01.1: schema con `normalized_hash`, `duplicate_count`, `last_seen_at` en tabla observations
- [ ] HU-01.2: CRUD base `SaveObservation` con capaciad de persistir `NormalizedHash`
