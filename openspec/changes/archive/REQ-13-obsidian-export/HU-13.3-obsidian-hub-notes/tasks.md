# Tasks: HU-13.3-obsidian-hub-notes

## Backend

- [ ] **B1: Crear hub.go con SessionHubFrontmatter y TopicHubFrontmatter**
      ```go
      type SessionHubFrontmatter struct {
          Type      string   `yaml:"type"`
          SessionID string   `yaml:"session_id"`
          Project   string   `yaml:"project,omitempty"`
          Directory string   `yaml:"directory,omitempty"`
          Tags      []string `yaml:"tags"`
          CreatedAt string   `yaml:"created_at"`
          EndedAt   string   `yaml:"ended_at,omitempty"`
      }

      type TopicHubFrontmatter struct {
          Type         string   `yaml:"type"`
          TopicPrefix  string   `yaml:"topic_prefix"`
          Project      string   `yaml:"project,omitempty"`
          ObsCount     int      `yaml:"observation_count"`
          SessionCount int      `yaml:"session_count"`
          Tags         []string `yaml:"tags"`
      }
      ```

- [ ] **B2: Implementar GenerateSessionHubs**
      - Listar sesiones
      - Por cada sesión, listar observaciones
      - Si len(observations) == 0 → skip
      - Construir frontmatter + body con metadata
      - Generar wikilinks usando slugMap
      - Escribir `_sessions/{id}.md`

- [ ] **B3: Implementar GenerateTopicHubs**
      - Listar observaciones
      - Agrupar por topic_key
      - Filtrar grupos con len < 2
      - Construir frontmatter + body
      - Agrupar observaciones por type dentro del body
      - Escribir `_topics/{prefix}.md`

- [ ] **B4: Integrar hub notes en Export pipeline**
      - Agregar `IncludeHubNotes bool` a ExportOpts (default true)
      - Llamar GenerateSessionHubs y GenerateTopicHubs al final de Export
      - Ambos reciben slugMap compartido

- [ ] **B5: Agregar flag --include-hub-notes al CLI**
      - Default: true
      - `--include-hub-notes=false` para saltar generación de hubs

## Tests

- [ ] **T1: TestSessionHub — genera _sessions/{id}.md con wikilinks**
      ```go
      func TestSessionHub(t *testing.T) {
          reader := NewMockReader()
          sessionID := "session-abc"
          reader.AddSession(Session{ID: sessionID, Project: "Domain", Directory: "/tmp"})
          id1 := reader.AddObservation(Observation{ID: 1, Title: "Bug login", SessionID: sessionID, Type: "fix"})
          id2 := reader.AddObservation(Observation{ID: 2, Title: "Fix timezone", SessionID: sessionID, Type: "fix"})

          vault := t.TempDir()
          slugMap := map[int64]string{1: "bug-login", 2: "fix-timezone"}
          err := GenerateSessionHubs(context.Background(), reader, vault, slugMap)
          require.NoError(t, err)

          content, err := os.ReadFile(filepath.Join(vault, "_sessions", sessionID+".md"))
          require.NoError(t, err)
          assert.Contains(t, string(content), "type: session-hub")
          assert.Contains(t, string(content), "[[observations/bug-login]]")
          assert.Contains(t, string(content), "[[observations/fix-timezone]]")
      }
      ```

- [ ] **T2: TestSessionHubEmpty — sesión sin obs no genera archivo**
- [ ] **T3: TestSessionHubFrontmatter — YAML frontmatter correcto**

- [ ] **T4: TestTopicHub — genera _topics/{prefix}.md con wikilinks**
      ```go
      func TestTopicHub(t *testing.T) {
          reader := NewMockReader()
          reader.AddObservation(Observation{ID: 1, Title: "Bug auth", TopicKey: "auth", Type: "fix"})
          reader.AddObservation(Observation{ID: 2, Title: "Login fix", TopicKey: "auth", Type: "fix"})
          reader.AddObservation(Observation{ID: 3, Title: "JWT impl", TopicKey: "auth", Type: "feat"})

          vault := t.TempDir()
          slugMap := map[int64]string{1: "bug-auth", 2: "login-fix", 3: "jwt-impl"}
          err := GenerateTopicHubs(context.Background(), reader, vault, slugMap)
          require.NoError(t, err)

          content, err := os.ReadFile(filepath.Join(vault, "_topics", "auth.md"))
          require.NoError(t, err)
          assert.Contains(t, string(content), "type: topic-hub")
          assert.Contains(t, string(content), "topic_prefix: auth")
          assert.Contains(t, string(content), "[[observations/bug-auth]]")
      }
      ```

- [ ] **T5: TestTopicHubThreshold — topic con 1 obs no genera archivo**
      ```go
      func TestTopicHubThreshold(t *testing.T) {
          reader := NewMockReader()
          reader.AddObservation(Observation{ID: 1, Title: "Solo", TopicKey: "solo", Type: "fix"})

          vault := t.TempDir()
          err := GenerateTopicHubs(context.Background(), reader, vault, map[int64]string{})
          require.NoError(t, err)

          _, err = os.Stat(filepath.Join(vault, "_topics", "solo.md"))
          assert.True(t, os.IsNotExist(err))
      }
      ```

- [ ] **T6: TestTopicHubGroupByType — observaciones agrupadas por type**
      ```go
      func TestTopicHubGroupByType(t *testing.T) {
          reader := NewMockReader()
          reader.AddObservation(Observation{ID: 1, Title: "Fix auth", TopicKey: "auth", Type: "fix"})
          reader.AddObservation(Observation{ID: 2, Title: "Feat auth", TopicKey: "auth", Type: "feat"})

          vault := t.TempDir()
          slugMap := map[int64]string{1: "fix-auth", 2: "feat-auth"}
          err := GenerateTopicHubs(context.Background(), reader, vault, slugMap)
          require.NoError(t, err)

          content, _ := os.ReadFile(filepath.Join(vault, "_topics", "auth.md"))
          assert.Contains(t, string(content), "## fix")
          assert.Contains(t, string(content), "## feat")
      }
      ```

- [ ] **T7: TestTopicHubFrontmatter — YAML frontmatter con observation_count y session_count**

- [ ] **T8: Sabotaje — no chequear threshold de 2 en topic hubs**
      1. Eliminar condición `if len(group) < 2 { continue }`
      2. Ejecutar `TestTopicHubThreshold` → falla (archivo se crea)
      3. Restaurar condición
      4. Test pasa

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/obsidian/... -v -count=1` — suite verde
- [ ] Verificar que _sessions/ y _topics/ aparecen en vault
- [ ] Commit: `feat: session and topic hub notes for obsidian vault`
