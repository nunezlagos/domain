# Tasks: issue-02.3-passive-capture

## Backend

- [ ] **B1: Crear `internal/store/capture.go` con tipos y errores**
      - Variables error: `ErrNoLearningsSection`, `ErrEmptyLearningsSection`
      - Constantes para prefijos: bulletPrefix, numberedPrefix, checklistPrefixes

- [ ] **B2: Implementar `ExtractLearnings(text string) ([]string, error)`**
      - Máquina de estados: searching, inSection, inCodeBlock
      - Split en líneas con `strings.Split(text, "\n")`
      - Detectar `^## Key Learnings:$` (case-sensitive exacto)
      - Recolectar líneas que matcheen patrones de items:
        - `^- (.+)` → bullet
        - `^- \[\S\] (.+)` → checklist
        - `^\d+[\.\)] (.+)` → numbered
      - Ignorar líneas dentro de code blocks (```)
      - Si no hay sección → `ErrNoLearningsSection`
      - Si hay sección pero 0 items → `ErrEmptyLearningsSection`
      - Si múltiples secciones, combinar items de todas

- [ ] **B3: Definir interfaz `observationWriter`**
      ```go
      type observationWriter interface {
          AddObservation(ctx context.Context, o Observation) (int64, error)
          ExistsByContent(ctx context.Context, sessionID, content string) (bool, error)
      }
      ```

- [ ] **B4: Implementar `CapturePassive(store, ctx, sessionID, text) (int, error)`**
      - Llamar `ExtractLearnings(text)`
      - Por cada item: check `ExistsByContent` → si existe, skip
      - Si no existe: `AddObservation` con type="learning", scope="session"
      - Retornar cantidad de observaciones nuevas creadas
      - Si ocurre error en mitad del batch, retornar count parcial + error

## Frontend

- [ ] N/A — HU puramente backend

## Tests

- [ ] **T1: TestExtractBullets — bullets simples**
      ```go
      func TestExtractBullets(t *testing.T) {
          text := "## Key Learnings:\n- aprendizaje uno\n- aprendizaje dos\n- aprendizaje tres"
          items, err := ExtractLearnings(text)
          require.NoError(t, err)
          assert.Equal(t, 3, len(items))
          assert.Equal(t, "aprendizaje uno", items[0])
      }
      ```

- [ ] **T2: TestExtractNumbered — items numerados**
      ```go
      func TestExtractNumbered(t *testing.T) {
          text := "## Key Learnings:\n1. primero\n2. segundo\n"
          items, err := ExtractLearnings(text)
          require.NoError(t, err)
          assert.Equal(t, 2, len(items))
          assert.Equal(t, "primero", items[0])
      }
      ```

- [ ] **T3: TestExtractChecklist — checklists con y sin check**
      ```go
      func TestExtractChecklist(t *testing.T) {
          text := "## Key Learnings:\n- [x] completado\n- [ ] pendiente\n"
          items, err := ExtractLearnings(text)
          require.NoError(t, err)
          assert.Equal(t, "completado", items[0])
          assert.Equal(t, "pendiente", items[1])
      }
      ```

- [ ] **T4: TestExtractNoSection — sin sección**
      ```go
      func TestExtractNoSection(t *testing.T) {
          text := "## Otro encabezado\nalgo de texto\n"
          _, err := ExtractLearnings(text)
          assert.ErrorIs(t, err, ErrNoLearningsSection)
      }
      ```

- [ ] **T5: TestExtractEmptySection — sección vacía**
      ```go
      func TestExtractEmptySection(t *testing.T) {
          text := "## Key Learnings:\n"
          _, err := ExtractLearnings(text)
          assert.ErrorIs(t, err, ErrEmptyLearningsSection)
      }
      ```

- [ ] **T6: TestExtractIgnoreCodeBlock — ignora sección dentro de code block**
      ```go
      func TestExtractIgnoreCodeBlock(t *testing.T) {
          text := "```\n## Key Learnings:\n- item dentro de code\n```\n## Key Learnings:\n- item real\n"
          items, err := ExtractLearnings(text)
          require.NoError(t, err)
          assert.Equal(t, 1, len(items))
          assert.Equal(t, "item real", items[0])
      }
      ```

- [ ] **T7: TestExtractMultipleSections — dos secciones**
      ```go
      func TestExtractMultipleSections(t *testing.T) {
          text := "## Key Learnings:\n- a\n## Key Learnings:\n- b\n"
          items, err := ExtractLearnings(text)
          require.NoError(t, err)
          assert.Equal(t, 2, len(items))
      }
      ```

- [ ] **T8: TestCapturePassiveDedup — duplicados se skipean**
      ```go
      func TestCapturePassiveDedup(t *testing.T) {
          db := setupTestDB(t)
          store := NewObservationStore(db)
          sess, _ := NewSessionStore(db).Start(ctx, "s1", "p1", "/d1")
          text := "## Key Learnings:\n- item unico\n- item duplicado\n"
          n1, err := CapturePassive(store, ctx, sess.ID, text)
          require.NoError(t, err)
          assert.Equal(t, 2, n1)
          n2, err := CapturePassive(store, ctx, sess.ID, text)
          require.NoError(t, err)
          assert.Equal(t, 0, n2)
      }
      ```

- [ ] **T9: TestExtractMixedFormats — mezcla de todos los formatos**
      ```go
      func TestExtractMixedFormats(t *testing.T) {
          text := "## Key Learnings:\n- bullet\n1. numbered\n- [x] checklist\n* asterisk\n"
          items, err := ExtractLearnings(text)
          require.NoError(t, err)
          assert.Equal(t, 4, len(items))
      }
      ```

- [ ] **T10: Sabotaje — item con solo whitespace**
      1. En el parser, comentar el `strings.TrimSpace` del contenido
      2. Pasar texto con item "   " → debe crear observación vacía
      3. Test `TestExtractEmptyItem` debe fallar
      4. Restaurar TrimSpace
      5. Ejecutar test → pasa
      6. Documentar sabotaje

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/store/... -v` — suite completa verde
- [ ] Commit: `feat: add passive capture of key learnings from text`
