# Tasks: issue-13.1-obsidian-exporter

## Backend

- [ ] **B1: Crear paquete `internal/obsidian/` con estructura de archivos**
      - `exporter.go` — Export pipeline
      - `types.go` — ExportOpts, Note, NoteFrontmatter, VaultState
      - `reader.go` — StoreReader interface
      - `frontmatter.go` — YAML frontmatter builder
      - `wikilink.go` — Wikilink generator
      - `slug.go` — Slugify + collision resolver

- [ ] **B2: Implementar StoreReader interface**
      ```go
      type StoreReader interface {
          ListObservations(ctx context.Context, filter ObservationFilter) ([]Observation, error)
          GetObservation(ctx context.Context, id int64) (Observation, error)
          ListSessions(ctx context.Context) ([]Session, error)
          GetSession(ctx context.Context, id string) (Session, error)
          ListPrompts(ctx context.Context, filter PromptFilter) ([]Prompt, error)
          GetPrompt(ctx context.Context, id int64) (Prompt, error)
          ListRelationsForObservation(ctx context.Context, id int64) ([]Relation, error)
          ListObservationsBySession(ctx context.Context, sessionID string) ([]Observation, error)
      }
      ```

- [ ] **B3: Implementar slugify y desambiguación**
      - `slugify(title string) string` — lowercase, replace spaces, strip non-alphanumeric, collapse dashes
      - `SlugResolver` — struct con mapa `slug → count` para desambiguación en batch
      - `Resolve(title string) (slug string)` — si colisión, append `-2`, `-3`, etc

- [ ] **B4: Implementar Note y NoteFrontmatter**
      ```go
      type NoteFrontmatter struct {
          ID        int64    `yaml:"id"`
          Type      string   `yaml:"type"`
          Title     string   `yaml:"title"`
          Content   string   `yaml:"content"`
          Project   string   `yaml:"project"`
          Scope     string   `yaml:"scope"`
          SessionID string   `yaml:"session_id,omitempty"`
          TopicKey  string   `yaml:"topic_key,omitempty"`
          CreatedAt string   `yaml:"created_at"`
          UpdatedAt string   `yaml:"updated_at"`
          Tags      []string `yaml:"tags,omitempty"`
          Aliases   []string `yaml:"aliases,omitempty"`
      }

      type Note struct {
          Frontmatter NoteFrontmatter
          Body        string
          Wikilinks   []string
      }

      func (n Note) Render() (string, error) — genera el string .md completo con frontmatter + body + wikilinks
      ```

- [ ] **B5: Implementar frontmatter rendering**
      - Usar `gopkg.in/yaml.v3` para marshal del struct
      - Encerrar entre `---` separadores
      - Escapar strings correctamente

- [ ] **B6: Implementar wikilink generation**
      - `GenerateWikilinks(obsID int64, relations []Relation, slugMap map[int64]string) []string`
      - Por cada relación, determinar el target (la otra punta)
      - Buscar slug de target en slugMap
      - Formato: `[[observations/{slug}]]`
      - Si target no está en slugMap (no se exportó), generar igual — será Dead Link

- [ ] **B7: Implementar Export pipeline**
      ```go
      func Export(ctx context.Context, reader StoreReader, opts ExportOpts) (*ExportReport, error)
      ```
      - Validar opts.VaultPath != ""
      - Asegurar que existe vault directory + subdirectorios (observations/, sessions/, prompts/)
      - Leer state file (o empezar vacío)
      - Construir ObservationFilter desde opts
      - Listar observaciones
      - Para cada obs:
        - Resolver slug
        - Si ya exportada y !opts.Force → skip
        - Obtener relaciones → wikilinks
        - Renderizar Note
        - Escribir archivo
      - Si opts.IncludeSessions: exportar sesiones
      - Si opts.IncludePrompts: exportar prompts
      - Actualizar state file con last_export = now
      - Retornar ExportReport { FilesCreated, FilesSkipped, Errors }

- [ ] **B8: Integrar CLI command `engram obsidian export`**
      - Flags: `--vault`, `--project`, `--limit`, `--since`, `--force`, `--include-sessions`, `--include-prompts`
      - Cobra command en `cmd/obsidian.go` o dentro del comando `obsidian`

- [ ] **B9: Implementar ExportReport**
      ```go
      type ExportReport struct {
          FilesCreated int
          FilesSkipped int
          Errors       int
          Duration     string
      }
      ```

## Tests

- [ ] **T1: TestSlugify — título normal se convierte correctamente**
      ```go
      func TestSlugify(t *testing.T) {
          tests := []struct{ input, expected string }{
              {"Bug en login modal", "bug-en-login-modal"},
              {"  Spaces  around  ", "spaces-around"},
              {"Special! chars? (yes)", "special-chars-yes"},
              {"camelCase", "camelcase"},
              {"ACRÓNIMO", "acrónimo"},
              {"", ""},
          }
          for _, tc := range tests {
              assert.Equal(t, tc.expected, slugify(tc.input))
          }
      }
      ```

- [ ] **T2: TestSlugCollision — títulos iguales se desambiguan**
      ```go
      func TestSlugCollision(t *testing.T) {
          r := NewSlugResolver()
          s1 := r.Resolve("Bug fix")
          s2 := r.Resolve("Bug fix")
          s3 := r.Resolve("Bug fix")
          assert.Equal(t, "bug-fix", s1)
          assert.Equal(t, "bug-fix-2", s2)
          assert.Equal(t, "bug-fix-3", s3)
      }
      ```

- [ ] **T3: TestNoteRender — frontmatter + body + wikilinks se renderizan correctamente**
      ```go
      func TestNoteRender(t *testing.T) {
          note := Note{
              Frontmatter: NoteFrontmatter{
                  ID: 1, Type: "fix", Title: "Bug login",
                  Content: "El modal no cierra", Project: "Domain",
                  CreatedAt: "2026-06-07T10:00:00Z",
              },
              Body: "## Context\nEl modal no cierra al hacer submit.",
              Wikilinks: []string{"[[observations/timezone-fix]]"},
          }
          result, err := note.Render()
          require.NoError(t, err)
          assert.Contains(t, result, "---")
          assert.Contains(t, result, "id: 1")
          assert.Contains(t, result, "type: fix")
          assert.Contains(t, result, "[[observations/timezone-fix]]")
          assert.Contains(t, result, "## Context")
      }
      ```

- [ ] **T4: TestExportBasic — exporta N observaciones a directorio temporal**
      ```go
      func TestExportBasic(t *testing.T) {
          reader := NewMockReader()
          reader.AddObservation(Observation{ID: 1, Title: "Bug login", Type: "fix", ...})
          reader.AddObservation(Observation{ID: 2, Title: "Fix timezone", Type: "fix", ...})

          vault := t.TempDir()
          opts := ExportOpts{VaultPath: vault, Force: true}
          report, err := Export(context.Background(), reader, opts)
          require.NoError(t, err)
          assert.Equal(t, 2, report.FilesCreated)

          files, _ := filepath.Glob(filepath.Join(vault, "observations", "*.md"))
          assert.Len(t, files, 2)
      }
      ```

- [ ] **T5: TestExportFilterProject — solo exporta observaciones del project especificado**
- [ ] **T6: TestExportFilterSince — solo exporta observaciones recientes**
- [ ] **T7: TestExportFilterLimit — exporta máximo N observaciones**
- [ ] **T8: TestExportForce — sobrescribe archivos existentes**
- [ ] **T9: TestExportNoForce — no sobrescribe archivos existentes**
- [ ] **T10: TestExportIncludeSessions — exporta sesiones con wikilinks**
- [ ] **T11: TestExportIncludePrompts — exporta prompts**
- [ ] **T12: TestExportVaultRequired — error sin --vault**
- [ ] **T13: TestWikilinkGeneration — obs con relación genera wikilink correcto**
- [ ] **T14: TestWikilinkToUnexportedObs — wikilink a obs no exportada igual se genera**
- [ ] **T15: TestMockReader — StoreReader mock implementa interfaz correctamente**
      ```go
      type MockReader struct {
          observations map[int64]Observation
          relations    map[int64][]Relation
          sessions     map[string]Session
          prompts      map[int64]Prompt
      }
      ```

- [ ] **T16: Sabotaje — desactivar desambiguación en SlugResolver**
      1. Comentar el chequeo de colisión en `Resolve`
      2. Ejecutar `TestSlugCollision` → falla (slug duplicado)
      3. Restaurar chequeo
      4. Test pasa nuevamente
      5. Documentar sabotaje

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/obsidian/... -v -count=1` — suite completa verde
- [ ] Verificar que StoreReader no expone métodos de escritura (compile-time check)
- [ ] Verificar que --vault sin valor da error claro
- [ ] Commit: `feat: obsidian markdown exporter with wikilinks and YAML frontmatter`
