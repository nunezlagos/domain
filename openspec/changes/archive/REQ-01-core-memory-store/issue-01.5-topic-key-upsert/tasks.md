# Tasks: issue-01.5-topic-key-upsert

## Backend

- [ ] **B1: Extender interfaz Store con UpsertByTopicKey**
      - Agregar método `UpsertByTopicKey(ctx context.Context, obs Observation) (SaveResult, error)` en `internal/store/store.go`
      - Extender `SaveResult` con campo `Updated bool`

- [ ] **B2: Extender SaveResult con campo Updated**
      - `internal/store/store.go`:
        ```go
        type SaveResult struct {
            Observation  Observation
            Deduplicated bool
            OriginalID   int64
            Updated      bool   // true si fue upsert (issue-01.5)
        }
        ```

- [ ] **B3: Implementar UpsertByTopicKey en store base**
      - `internal/store/store.go` — método en el struct `store`:
        1. Si `obs.TopicKey == ""` → delegar a `SaveObservation` (insert normal)
        2. Iniciar transacción
        3. Query existente:
           ```sql
           SELECT id, revision_count FROM observations
           WHERE topic_key = ? AND project = ? AND scope = ?
           AND deleted_at IS NULL
           ORDER BY updated_at DESC LIMIT 1
           ```
        4. Si existe → UPDATE:
           ```sql
           UPDATE observations
           SET title = ?, content = ?, revision_count = ?, updated_at = datetime('now')
           WHERE id = ?
           ```
           donde `revision_count = existingRevision + 1`
        5. Si no existe → INSERT con `revision_count = 1`
        6. Commit transacción
        7. Retornar `SaveResult{Observation: result, Updated: true/false}`

- [ ] **B4: Garantizar que DeduplicatingStore compone correctamente con UpsertByTopicKey**
      - En `DeduplicatingStore.SaveObservation` (issue-01.4), si `obs.TopicKey != ""`, delegar a `base.UpsertByTopicKey` en lugar de aplicar dedup
      - Agregar delegación explícita para evitar doble procesamiento

- [ ] **B5: Actualizar documentación de la interfaz Store**
      - Comentario Go doc en `UpsertByTopicKey` explicando:
        - Qué campos se usan para matching (topic_key + project + scope)
        - Qué campos se actualizan (title, content)
        - Qué NO se actualiza (type, tool_name, session_id)

## Frontend

- [ ] N/A — HU puramente backend

## Tests

- [ ] **T1: TestUpsertNewTopicKey — nuevo topic_key crea registro**
      ```go
      func TestUpsertNewTopicKey(t *testing.T) {
          db := setupTestDB(t)
          store := NewStore(db, Config{})

          obs := Observation{
              SessionID: "s1", Project: "p1", Scope: "project",
              TopicKey: "tk:first", Title: "First", Content: "hello",
          }
          result, err := store.UpsertByTopicKey(ctx, obs)
          require.NoError(t, err)
          assert.False(t, result.Updated)
          assert.Equal(t, 1, result.Observation.RevisionCount)
      }
      ```

- [ ] **T2: TestUpsertExistingTopicKey — mismo topic_key actualiza**
      ```go
      func TestUpsertExistingTopicKey(t *testing.T) {
          db := setupTestDB(t)
          store := NewStore(db, Config{})

          obs1 := Observation{SessionID: "s1", Project: "p1", Scope: "project",
              TopicKey: "tk:goal", Title: "Old title", Content: "old content"}
          r1, _ := store.UpsertByTopicKey(ctx, obs1)

          obs2 := Observation{SessionID: "s1", Project: "p1", Scope: "project",
              TopicKey: "tk:goal", Title: "New title", Content: "new content"}
          r2, err := store.UpsertByTopicKey(ctx, obs2)
          require.NoError(t, err)

          assert.True(t, r2.Updated)
          assert.Equal(t, 2, r2.Observation.RevisionCount)
          assert.Equal(t, "New title", r2.Observation.Title)
          assert.Equal(t, "new content", r2.Observation.Content)
          assert.Equal(t, r1.Observation.ID, r2.Observation.ID) // mismo registro
      }
      ```

- [ ] **T3: TestUpsertDifferentProject — mismo topic_key, diferente project**
      ```go
      func TestUpsertDifferentProject(t *testing.T) {
          db := setupTestDB(t)
          store := NewStore(db, Config{})

          obs1 := Observation{SessionID: "s1", Project: "proj-a", Scope: "project",
              TopicKey: "tk:goal", Title: "Goal A", Content: "content"}
          r1, _ := store.UpsertByTopicKey(ctx, obs1)

          obs2 := Observation{SessionID: "s1", Project: "proj-b", Scope: "project",
              TopicKey: "tk:goal", Title: "Goal B", Content: "content"}
          r2, _ := store.UpsertByTopicKey(ctx, obs2)

          assert.False(t, r2.Updated)
          assert.NotEqual(t, r1.Observation.ID, r2.Observation.ID)
      }
      ```

- [ ] **T4: TestUpsertDifferentScope — mismo topic_key, diferente scope**
      ```go
      func TestUpsertDifferentScope(t *testing.T) {
          db := setupTestDB(t)
          store := NewStore(db, Config{})

          obs1 := Observation{SessionID: "s1", Project: "p1", Scope: "project",
              TopicKey: "tk:goal", Title: "Project goal", Content: "content"}
          r1, _ := store.UpsertByTopicKey(ctx, obs1)

          obs2 := Observation{SessionID: "s1", Project: "p1", Scope: "personal",
              TopicKey: "tk:goal", Title: "Personal goal", Content: "content"}
          r2, _ := store.UpsertByTopicKey(ctx, obs2)

          assert.False(t, r2.Updated)
          assert.NotEqual(t, r1.Observation.ID, r2.Observation.ID)
      }
      ```

- [ ] **T5: TestUpsertEmptyTopicKey — topic_key vacío hace insert normal**
      ```go
      func TestUpsertEmptyTopicKey(t *testing.T) {
          db := setupTestDB(t)
          store := NewStore(db, Config{})

          obs := Observation{SessionID: "s1", Project: "p1", Scope: "project",
              TopicKey: "", Title: "No topic", Content: "content"}
          result, err := store.UpsertByTopicKey(ctx, obs)
          require.NoError(t, err)
          assert.False(t, result.Updated)
          assert.Equal(t, 1, result.Observation.RevisionCount)
      }
      ```

- [ ] **T6: TestUpsertMultipleTimes — revision_count se incrementa en cada upsert**
      ```go
      func TestUpsertMultipleTimes(t *testing.T) {
          db := setupTestDB(t)
          store := NewStore(db, Config{})

          obs := Observation{SessionID: "s1", Project: "p1", Scope: "project",
              TopicKey: "tk:iter", Title: "Iter 1", Content: "v1"}
          r1, _ := store.UpsertByTopicKey(ctx, obs)
          assert.Equal(t, 1, r1.Observation.RevisionCount)

          obs.Title = "Iter 2"
          obs.Content = "v2"
          r2, _ := store.UpsertByTopicKey(ctx, obs)
          assert.Equal(t, 2, r2.Observation.RevisionCount)

          obs.Title = "Iter 3"
          obs.Content = "v3"
          r3, _ := store.UpsertByTopicKey(ctx, obs)
          assert.Equal(t, 3, r3.Observation.RevisionCount)

          // Todos apuntan al mismo ID
          assert.Equal(t, r1.Observation.ID, r2.Observation.ID)
          assert.Equal(t, r2.Observation.ID, r3.Observation.ID)
      }
      ```

- [ ] **T7: TestUpsertWithSoftDeleted — registro soft-deleteado no se actualiza, crea nuevo**
      ```go
      func TestUpsertWithSoftDeleted(t *testing.T) {
          db := setupTestDB(t)
          store := NewStore(db, Config{})

          obs := Observation{SessionID: "s1", Project: "p1", Scope: "project",
              TopicKey: "tk:del", Title: "Original", Content: "content"}
          r1, _ := store.UpsertByTopicKey(ctx, obs)

          // Soft-delete
          err := store.SoftDeleteObservation(ctx, r1.Observation.ID)
          require.NoError(t, err)

          // Upsert con mismo topic_key
          r2, err := store.UpsertByTopicKey(ctx, obs)
          require.NoError(t, err)

          assert.False(t, r2.Updated) // nuevo registro
          assert.NotEqual(t, r1.Observation.ID, r2.Observation.ID)
          assert.Equal(t, 1, r2.Observation.RevisionCount)
      }
      ```

- [ ] **T8: TestUpsertTransactionalRace — concurrencia no duplica**
      ```go
      func TestUpsertTransactionalRace(t *testing.T) {
          db := setupTestDB(t)
          store := NewStore(db, Config{DedupWindow: 0})

          var wg sync.WaitGroup
          for i := 0; i < 5; i++ {
              wg.Add(1)
              go func() {
                  defer wg.Done()
                  obs := Observation{SessionID: "s1", Project: "p1", Scope: "project",
                      TopicKey: "tk:race", Title: "Race", Content: "content"}
                  _, err := store.UpsertByTopicKey(ctx, obs)
                  assert.NoError(t, err)
              }()
          }
          wg.Wait()

          var count int
          db.QueryRow(
              "SELECT COUNT(*) FROM observations WHERE topic_key = 'tk:race' AND project = 'p1' AND scope = 'project' AND deleted_at IS NULL",
          ).Scan(&count)
          assert.Equal(t, 1, count) // solo 1 registro, no 5
      }
      ```

- [ ] **T9: TestDedupAndUpsertComposition — DeduplicatingStore usa UpsertByTopicKey cuando hay topic_key**
      ```go
      func TestDedupAndUpsertComposition(t *testing.T) {
          db := setupTestDB(t)
          store := NewStore(db, Config{DedupWindow: 60 * time.Second})

          // Con topic_key → upsert, no dedup
          obs := Observation{SessionID: "s1", Project: "p1", Scope: "project",
              TopicKey: "tk:compose", Title: "V1", Content: "content"}
          r1, _ := store.SaveObservation(ctx, obs)
          assert.False(t, r1.Deduplicated)

          // Mismo hash pero con topic_key → upsert (no dedup)
          obs.Title = "V2"
          r2, _ := store.SaveObservation(ctx, obs)
          assert.True(t, r2.Updated)   // es upsert
          assert.False(t, r2.Deduplicated) // no es dedup
          assert.Equal(t, r1.Observation.ID, r2.Observation.ID)
          assert.Equal(t, 2, r2.Observation.RevisionCount)

          // Sin topic_key → dedup normal
          obsNoKey := Observation{SessionID: "s1", Project: "p1", Scope: "project",
              TopicKey: "", Title: "No key", Content: "same"}
          r3, _ := store.SaveObservation(ctx, obsNoKey)
          r4, _ := store.SaveObservation(ctx, obsNoKey)
          assert.True(t, r4.Deduplicated)
      }
      ```

- [ ] **T10: Sabotaje — comentar SELECT de existencia → upsert siempre inserta**
      1. En `UpsertByTopicKey`, comentar la query SELECT y siempre ir al branch INSERT
      2. Ejecutar `TestUpsertExistingTopicKey` → debe fallar (no actualiza, inserta duplicado)
      3. Restaurar la query SELECT
      4. Ejecutar `TestUpsertExistingTopicKey` nuevamente → debe pasar
      5. Documentar el sabotaje en comentario del test

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/store/... -v` — suite completa verde
- [ ] Verificar cobertura de todos los scenarios Gherkin en tests
- [ ] Verificar que composición con DeduplicatingStore (issue-01.4) funciona correctamente
- [ ] Commit: `feat: add upsert by topic_key with revision_count semantics`

## Dependencias

- [ ] issue-01.1: schema con `topic_key`, `revision_count`, `updated_at` en tabla observations
- [ ] issue-01.2: CRUD base funcionando (GetObservation, SaveObservation)
- [ ] issue-01.4: interfaz Store consolidada y SaveResult struct con campos existentes
