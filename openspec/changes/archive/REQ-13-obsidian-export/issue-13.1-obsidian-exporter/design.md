# Design: issue-13.1-obsidian-exporter

## Decisión arquitectónica

### StoreReader interface

```go
// StoreReader es la interfaz read-only que el exporter necesita.
// Garantiza que el exportador nunca escribe en la store de memoria.
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

El package `internal/store` implementa `StoreReader` implícitamente. El exporter recibe un `StoreReader` en su constructor, nunca un `*sql.DB` directamente.

### Export pipeline

```
1. Parsear flags → ExportOpts struct
2. Validar: --vault es requerido, path existe
3. Cargar state (issue-13.4): last_export timestamp, exported slugs map
4. Listar observaciones con filtros (project, since, limit)
5. Para cada observación:
   a. Calcular slug (determinístico desde title)
   b. Si colisión con slug existente → desambiguar (-2, -3, etc)
   c. Si ya exportada y !force → skip
   d. Obtener relaciones → generar wikilinks
   e. Construir Note (frontmatter + body + wikilinks)
   f. Escribir {vault}/observations/{slug}.md
6. Si --include-sessions: exportar sesiones
7. Si --include-prompts: exportar prompts
8. Actualizar state file con nueva last_export y slugs
```

### Note structure

```go
type Note struct {
    Frontmatter NoteFrontmatter
    Body        string
    Wikilinks   []string
}

type NoteFrontmatter struct {
    ID        int64  `yaml:"id"`
    Type      string `yaml:"type"`
    Title     string `yaml:"title"`
    Content   string `yaml:"content"`
    Project   string `yaml:"project"`
    Scope     string `yaml:"scope"`
    SessionID string `yaml:"session_id,omitempty"`
    TopicKey  string `yaml:"topic_key,omitempty"`
    CreatedAt string `yaml:"created_at"`
    UpdatedAt string `yaml:"updated_at"`
    Tags      []string `yaml:"tags,omitempty"`
    Aliases   []string `yaml:"aliases,omitempty"`
}
```

### Slug generation

```go
func slugify(title string) string {
    s := strings.ToLower(title)
    s = strings.TrimSpace(s)
    s = strings.ReplaceAll(s, " ", "-")
    reg := regexp.MustCompile(`[^a-z0-9-]`)
    s = reg.ReplaceAllString(s, "")
    s = regexp.MustCompile(`-+`).ReplaceAllString(s, "-")
    s = strings.Trim(s, "-")
    return s
}
```

Desambiguación: mapa global `slugCounts map[string]int` durante el export batch. Si `slugify(title)` ya existe en el mapa, append `-{count+1}`. Al terminar el batch, el mapa se guarda en state para persistencia entre exports.

### Wikilink generation

Para cada observación, se consultan sus relaciones en `memory_relations`:

```go
func generateWikilinks(relations []Relation) []string {
    var links []string
    for _, r := range relations {
        // La relación puede ser source→target o target→source
        id := r.SourceID
        if id == obsID {
            id = r.TargetID
        }
        links = append(links, fmt.Sprintf("[[observations/%s]]", slugByID[id]))
    }
    return links
}
```

Los wikilinks se insertan al final del body markdown:

```md
## Relaciones

- [[observations/bug-en-login-modal]]
- [[observations/fix-timezone-handling]]
```

### Vault directory structure

```
{vault}/
├── .engram-state.yaml
├── observations/
│   ├── bug-en-login-modal.md
│   ├── fix-timezone-handling.md
│   └── ...
├── sessions/
│   ├── session-abc123.md
│   └── ...
└── prompts/
    ├── prompt-42.md
    └── ...
```

### ExportOpts

```go
type ExportOpts struct {
    VaultPath       string
    Project         string
    Limit           int
    Since           time.Duration
    Force           bool
    IncludeSessions bool
    IncludePrompts  bool
}
```

### CLI integration

```go
var exportCmd = &cobra.Command{
    Use:   "export",
    Short: "Export memories to Obsidian vault",
    RunE: func(cmd *cobra.Command, args []string) error {
        opts := ExportOpts{
            VaultPath:       viper.GetString("vault"),
            Project:         viper.GetString("project"),
            Limit:           viper.GetInt("limit"),
            Since:           viper.GetDuration("since"),
            Force:           viper.GetBool("force"),
            IncludeSessions: viper.GetBool("include-sessions"),
            IncludePrompts:  viper.GetBool("include-prompts"),
        }
        if opts.VaultPath == "" {
            return fmt.Errorf("flag --vault is required")
        }
        return obsidian.Export(ctx, reader, opts)
    },
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| UUID como filename | Slugs son legibles y recognoscibles en Obsidian; UUID rompe la experiencia de navegación |
| Un solo archivo por sesión con todas las obs | Cada observación como archivo individual permite wikilinks granulares y grafo de conocimiento |
| Template engine (pongo2, etc) | El formato es simple y estable; `fmt.Sprintf` + yaml.Marshal es suficiente |
| Exportar a notas embebidas en lugar de archivos separados | Obsidian funciona mejor con archivos individuales; el grafo se basa en archivos |
| Base de datos SQLite en vault | Archivos markdown planos portable, versionable con git, editable desde cualquier editor |

## TDD plan

1. **Red:** `TestSlugify` — título "Bug en login modal!" → espera "bug-en-login-modal" → falla
2. **Green:** Implementar slugify → pasa
3. **Red:** `TestSlugCollision` — dos títulos "Bug fix" → espera "bug-fix" y "bug-fix-2" → falla
4. **Green:** Implementar mapa de desambiguación → pasa
5. **Red:** `TestFrontmatter` — observación completa → espera YAML correcto → falla
6. **Green:** Implementar frontmatter rendering → pasa
7. **Red:** `TestWikilinks` — obs con relación → espera `[[observations/other-slug]]` → falla
8. **Green:** Implementar wikilink generation → pasa
9. **Red:** `TestExportToDir` — exportar N obs a temp dir → espera N archivos → falla
10. **Green:** Implementar pipeline completo → pasa
11. **Red:** `TestExportFilterProject` — exportar con --project → solo ese project → falla
12. **Green:** Implementar filtro project → pasa
13. **Red:** `TestExportFilterSince` → falla
14. **Green:** Implementar filtro since → pasa
15. **Sabotaje:** Desactivar desambiguación → test colisión falla → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Slugs no determinísticos si cambia título | Documentar que wikilinks son válidos hasta que se re-exporta; --force regenera todo; state file mantiene slug → id mapping |
| Domain con muchas observaciones (>10k) | Batch processing; --limit protege; streaming write a disco |
| Caracteres no ASCII en slugs | slugify elimina todo lo que no sea a-z0-9-; títulos en español pierden acentos pero siguen siendo legibles |
| Wikilinks rotos por cambios en slugs | state file mantiene slug history; si un slug cambia, se genera redirect note o se actualizan referencias con --force |
