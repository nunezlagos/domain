# Tasks: HU-01.7-privacy-stripping

## Backend

- [ ] **B1: Crear `internal/store/privacy.go` con stripPrivateTags**
      - Regex: `<private>.*?</private>` (non-greedy)
      - `ReplaceAllString(content, "[REDACTED]")`
      - Función exportada para que plugin layer también pueda usarla

- [ ] **B2: Integrar stripPrivateTags en AddObservation**
      - Al inicio del método, antes de cualquier validación:
        ```go
        observation.Content = stripPrivateTags(observation.Content)
        ```

- [ ] **B3: Integrar stripPrivateTags en AddPrompt**
      - Al inicio del método, antes de cualquier validación:
        ```go
        content = stripPrivateTags(content)
        ```

- [ ] **B4: Definir interfaz PrivacyStripper para plugin layer**
      - `type PrivacyStripper interface { StripPrivate(content string) string }`
      - En `internal/store/privacy.go` o en paquete compartido
      - No es necesario que el store implemente esta interfaz; es para consumo del plugin

## Frontend

- [ ] N/A — HU puramente backend

## Tests

- [ ] **T1: TestStripPrivateTags_Simple**
      ```go
      func TestStripPrivateTags_Simple(t *testing.T) {
          got := stripPrivateTags("<private>secret</private>")
          assert.Equal(t, "[REDACTED]", got)
      }
      ```

- [ ] **T2: TestStripPrivateTags_Multiple**
      ```go
      func TestStripPrivateTags_Multiple(t *testing.T) {
          got := stripPrivateTags("a<private>1</private>b<private>2</private>c")
          assert.Equal(t, "a[REDACTED]b[REDACTED]c", got)
      }
      ```

- [ ] **T3: TestStripPrivateTags_UnclosedTag**
      ```go
      func TestStripPrivateTags_UnclosedTag(t *testing.T) {
          input := "texto <private>abierto sin cerrar"
          got := stripPrivateTags(input)
          assert.Equal(t, input, got)
      }
      ```

- [ ] **T4: TestStripPrivateTags_NoOpeningTag**
      ```go
      func TestStripPrivateTags_NoOpeningTag(t *testing.T) {
          input := "text</private>"
          got := stripPrivateTags(input)
          assert.Equal(t, input, got)
      }
      ```

- [ ] **T5: TestStripPrivateTags_Nested**
      ```go
      func TestStripPrivateTags_Nested(t *testing.T) {
          got := stripPrivateTags("<private>outer<private>inner</private>tail</private>")
          assert.Equal(t, "[REDACTED]", got)
      }
      ```

- [ ] **T6: TestStripPrivateTags_AlreadyRedacted**
      ```go
      func TestStripPrivateTags_AlreadyRedacted(t *testing.T) {
          got := stripPrivateTags("[REDACTED]")
          assert.Equal(t, "[REDACTED]", got)
      }
      ```

- [ ] **T7: TestStripPrivateTags_NoTags**
      ```go
      func TestStripPrivateTags_NoTags(t *testing.T) {
          input := "texto normal sin tags"
          got := stripPrivateTags(input)
          assert.Equal(t, input, got)
      }
      ```

- [ ] **T8: TestStripPrivateTags_EmptyString**
      ```go
      func TestStripPrivateTags_EmptyString(t *testing.T) {
          got := stripPrivateTags("")
          assert.Equal(t, "", got)
      }
      ```

- [ ] **T9: TestAddObservation_StripsPrivate**
      ```go
      func TestAddObservation_StripsPrivate(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          s := &Store{db: db}
          obs := &Observation{
              SessionID: "session-1",
              Content:   "api key: <private>sk-123</private>",
          }
          id, err := s.AddObservation(context.Background(), obs)
          require.NoError(t, err)
          saved, _ := s.GetObservation(context.Background(), id)
          assert.NotContains(t, saved.Content, "sk-123")
          assert.Contains(t, saved.Content, "[REDACTED]")
      }
      ```

- [ ] **T10: TestAddPrompt_StripsPrivate**
      ```go
      func TestAddPrompt_StripsPrivate(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          s := &Store{db: db}
          id, err := s.AddPrompt(context.Background(), "session-1", "<private>token</private>", "test")
          require.NoError(t, err)
          p, _ := s.GetPrompt(context.Background(), id)
          assert.Equal(t, "[REDACTED]", p.Content)
      }
      ```

- [ ] **T11: TestDefenseInDepth_PluginLayerFallback**
      ```go
      func TestDefenseInDepth_PluginLayerFallback(t *testing.T) {
          // Simula que el plugin NO aplicó stripping
          content := "<private>sensitive</private>"
          // El store aplica stripping
          redacted := stripPrivateTags(content)
          assert.Equal(t, "[REDACTED]", redacted)
          // Doble aplicación es no-op
          double := stripPrivateTags(redacted)
          assert.Equal(t, "[REDACTED]", double)
      }
      ```

- [ ] **T12: Sabotaje — comentar stripPrivateTags en AddObservation → test cae → restaurar**
      1. En AddObservation, comentar `observation.Content = stripPrivateTags(observation.Content)`
      2. Ejecutar TestAddObservation_StripsPrivate → debe fallar (contiene "sk-123")
      3. Restaurar línea
      4. Ejecutar nuevamente → pasa
      5. Documentar sabotaje

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/store/... -v` — suite completa verde
- [ ] Commit: `feat: add privacy stripping with defense-in-depth at plugin and store layers`
