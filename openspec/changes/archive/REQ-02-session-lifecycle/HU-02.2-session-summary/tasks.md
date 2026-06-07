# Tasks: HU-02.2-session-summary

## Backend

- [ ] **B1: Definir struct `SessionSummary` y constantes de validación**
      - Struct con tags JSON: Goal, Instructions, Discoveries, Accomplished, NextSteps, RelevantFiles
      - Constantes `maxFieldLength = 10000`, `maxTotalBytes = 65535`
      - Variables error: `ErrSummaryValidation`, `ErrSummaryFieldTooLong`, `ErrSummarySessionEnded`, `ErrSummaryTooLarge`

- [ ] **B2: Implementar `Validate(summary *SessionSummary) error`**
      - Goal y Accomplished no vacíos (trim + len > 0)
      - Cada campo string ≤ 10000 runas (`utf8.RuneCountInString`)
      - JSON serializado ≤ 64KB
      - Retornar error específico según caso

- [ ] **B3: Implementar `SessionStore.SetSummary(ctx, sessionID, summary)`**
      - Llamar `Validate(summary)` primero
      - `SELECT status FROM sessions WHERE id = ?` — verificar que existe y está active
      - Si session not found → `ErrSessionNotFound`
      - Si status = completed → `ErrSummarySessionEnded`
      - `UPDATE sessions SET summary = ?, updated_at = ? WHERE id = ?`
      - Marshal summary a JSON string para almacenar

- [ ] **B4: Implementar `SessionStore.GetSummary(ctx, sessionID)`**
      - `SELECT summary FROM sessions WHERE id = ?`
      - Si no rows → `ErrSessionNotFound`
      - Si summary es NULL → retornar `nil, nil`
      - Unmarshal JSON a `*SessionSummary`
      - Si JSON corrupto → log warning + retornar `nil, nil` (sin panic)

## Frontend

- [ ] N/A — HU puramente backend (el summary se usa desde CLI/agent, no desde TUI directamente)

## Tests

- [ ] **T1: TestSummaryValidationRequired — campos requeridos**
      ```go
      func TestSummaryValidationRequired(t *testing.T) {
          err := Validate(&SessionSummary{Goal: "", Accomplished: "done"})
          assert.ErrorIs(t, err, ErrSummaryValidation)
          err = Validate(&SessionSummary{Goal: "task", Accomplished: ""})
          assert.ErrorIs(t, err, ErrSummaryValidation)
          err = Validate(&SessionSummary{Goal: "task", Accomplished: "done"})
          assert.NoError(t, err)
      }
      ```

- [ ] **T2: TestSummaryValidationFieldTooLong — campo excede límite**
      ```go
      func TestSummaryValidationFieldTooLong(t *testing.T) {
          long := strings.Repeat("a", maxFieldLength+1)
          err := Validate(&SessionSummary{Goal: long, Accomplished: "done"})
          assert.ErrorIs(t, err, ErrSummaryFieldTooLong)
      }
      ```

- [ ] **T3: TestSetAndGetSummary — guardar y recuperar**
      ```go
      func TestSetAndGetSummary(t *testing.T) {
          db := setupTestDB(t)
          store := NewSessionStore(db)
          sess, _ := store.Start(ctx, "s1", "p1", "/d1")
          summary := &SessionSummary{
              Goal:         "Implement X",
              Accomplished: "X implemented",
              Discoveries:  []string{"modernc es rápido"},
              NextSteps:    "Probar en CI",
          }
          err := store.SetSummary(ctx, sess.ID, summary)
          require.NoError(t, err)
          got, err := store.GetSummary(ctx, sess.ID)
          require.NoError(t, err)
          assert.Equal(t, summary.Goal, got.Goal)
          assert.Equal(t, summary.Accomplished, got.Accomplished)
          assert.Equal(t, summary.Discoveries, got.Discoveries)
      }
      ```

- [ ] **T4: TestSetSummaryEndedSession — sesión completada da error**
      ```go
      func TestSetSummaryEndedSession(t *testing.T) {
          db := setupTestDB(t)
          store := NewSessionStore(db)
          sess, _ := store.Start(ctx, "s1", "p1", "/d1")
          store.End(ctx, sess.ID, nil)
          err := store.SetSummary(ctx, sess.ID, &SessionSummary{Goal: "x", Accomplished: "y"})
          assert.ErrorIs(t, err, ErrSummarySessionEnded)
      }
      ```

- [ ] **T5: TestGetSummaryNonexistent — sesión no existe**
      ```go
      func TestGetSummaryNonexistent(t *testing.T) {
          db := setupTestDB(t)
          store := NewSessionStore(db)
          _, err := store.GetSummary(ctx, "no-session")
          assert.ErrorIs(t, err, ErrSessionNotFound)
      }
      ```

- [ ] **T6: TestGetSummaryNoSummary — sesión existe sin summary**
      ```go
      func TestGetSummaryNoSummary(t *testing.T) {
          db := setupTestDB(t)
          store := NewSessionStore(db)
          sess, _ := store.Start(ctx, "s1", "p1", "/d1")
          got, err := store.GetSummary(ctx, sess.ID)
          require.NoError(t, err)
          assert.Nil(t, got)
      }
      ```

- [ ] **T7: TestSummaryMarshalUnmarshal — serialización ida y vuelta**
      ```go
      func TestSummaryMarshalUnmarshal(t *testing.T) {
          original := &SessionSummary{
              Goal:          "Test",
              Accomplished:  "Done",
              RelevantFiles: []string{"a.go", "b.go"},
          }
          data, _ := json.Marshal(original)
          var decoded SessionSummary
          json.Unmarshal(data, &decoded)
          assert.Equal(t, original.Goal, decoded.Goal)
          assert.Equal(t, original.RelevantFiles, decoded.RelevantFiles)
      }
      ```

- [ ] **T8: TestSummaryValidationTooLarge — JSON total excede límite**
      ```go
      func TestSummaryValidationTooLarge(t *testing.T) {
          huge := strings.Repeat("x", maxTotalBytes)
          err := Validate(&SessionSummary{Goal: huge, Accomplished: "done"})
          assert.ErrorIs(t, err, ErrSummaryTooLarge)
      }
      ```

- [ ] **T9: Sabotaje — JSON corrupto en DB**
      1. `UPDATE sessions SET summary = '{corrupt: json' WHERE id = 's1'`
      2. Llamar `GetSummary` — no debe panic, debe retornar (nil, nil)
      3. Restaurar con JSON válido
      4. Verificar que GetSummary funciona nuevamente
      5. Documentar el sabotaje

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/store/... -v` — suite completa verde
- [ ] Commit: `feat: add structured session summary with validation`
