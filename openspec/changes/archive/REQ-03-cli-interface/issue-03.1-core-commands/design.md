# Design: issue-03.1-core-commands

## Decisión arquitectónica

### CLI entrypoint con Cobra

```
cmd/domain/main.go
├── rootCmd (--json, --project, --db-path flags globales)
├── saveCmd     → internal/cli/save.go
├── searchCmd   → internal/cli/search.go
├── deleteCmd   → internal/cli/delete.go
├── contextCmd  → internal/cli/context.go
├── statsCmd    → internal/cli/stats.go
└── versionCmd  → internal/cli/version.go
```

### Root command

```go
var rootCmd = &cobra.Command{
    Use:   "Domain",
    Short: "Domain - persistent memory for AI coding agents",
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        dbPath, _ := cmd.Flags().GetString("db-path")
        if dbPath == "" {
            dbPath = defaultDBPath()
        }
        db, err := store.InitDB(dbPath)
        if err != nil {
            return fmt.Errorf("cannot open database: %w", err)
        }
        // store db connection in context
        ctx := context.WithValue(cmd.Context(), ctxDB, db)
        cmd.SetContext(ctx)
        return nil
    },
}

func init() {
    rootCmd.PersistentFlags().Bool("json", false, "Output as JSON")
    rootCmd.PersistentFlags().String("project", "", "Override project name")
    rootCmd.PersistentFlags().String("db-path", defaultDBPath(), "Path to database")
    rootCmd.AddCommand(saveCmd, searchCmd, deleteCmd, contextCmd, statsCmd, versionCmd)
}
```

### Save command

```go
var saveCmd = &cobra.Command{
    Use:   "save <title> <content>",
    Short: "Save a new observation",
    Args:  cobra.ExactArgs(2),
    RunE: func(cmd *cobra.Command, args []string) error {
        flags := parseSaveFlags(cmd)
        obs := store.Observation{
            Title:   args[0],
            Content: args[1],
            Type:    flags.typ,
            Scope:   flags.scope,
            Project: resolveProject(flags.project),
            TopicKey: flags.topicKey,
        }
        var candidates []store.Candidate
        id, err := store.AddObservation(getDB(cmd), obs, false, &candidates)
        if err != nil { return err }
        if len(candidates) > 0 {
            cmd.PrintErrf("Warning: duplicate detected (similar ID %d)\n", candidates[0].ID)
        }
        output.Print(cmd, "created", map[string]any{"id": id})
        return nil
    },
}
```

Flags: `--type` (default "general"), `--scope` (default "project"), `--project`, `--topic-key`

### Search command

```go
var searchCmd = &cobra.Command{
    Use:   "search <query>",
    Short: "Search observations by text",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        filter := store.SearchFilter{
            Query:   args[0],
            Type:    getFlag(cmd, "type"),
            Project: getFlag(cmd, "project"),
            Scope:   getFlag(cmd, "scope"),
            Limit:   getFlagInt(cmd, "limit"),
        }
        results, err := store.SearchObservations(getDB(cmd), filter)
        if err != nil { return err }
        if len(results) == 0 {
            cmd.Println("No results found")
            return nil
        }
        output.PrintTable(cmd, results, []string{"ID", "Title", "Project", "Type", "Created"})
        return nil
    },
}
```

Flags: `--type`, `--project`, `--scope`, `--limit` (default 20)

### Delete command

```go
var deleteCmd = &cobra.Command{
    Use:   "delete <id>",
    Short: "Delete an observation (soft by default)",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        id, _ := strconv.ParseInt(args[0], 10, 64)
        hard, _ := cmd.Flags().GetBool("hard")
        err := store.DeleteObservation(getDB(cmd), id, hard)
        if err != nil { return err }
        msg := fmt.Sprintf("observation %d deleted", id)
        if hard { msg = fmt.Sprintf("observation %d permanently deleted", id) }
        cmd.Println(msg)
        return nil
    },
}
```

Flags: `--hard` (default false)

### Context command

```go
var contextCmd = &cobra.Command{
    Use:   "context [project]",
    Short: "Show current context: active session, project, recent observations",
    Args:  cobra.MaximumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        project := resolveProject("")
        if len(args) > 0 { project = args[0] }
        scope, _ := cmd.Flags().GetString("scope")

        // Get active session
        session, err := store.GetActiveSession(getDB(cmd), project)
        // Get recent observations
        recent, err := store.RecentObservations(getDB(cmd), store.ObservationFilter{
            Project: project, Scope: scope, Limit: 5,
        })

        output.Print(cmd, "context", map[string]any{
            "project": project,
            "session": session,
            "recent":  recent,
        })
        return nil
    },
}
```

Flags: `--scope`

### Stats command

```go
var statsCmd = &cobra.Command{
    Use:   "stats",
    Short: "Show global memory statistics",
    RunE: func(cmd *cobra.Command, args []string) error {
        stats, err := store.GetStats(getDB(cmd))
        if err != nil { return err }
        output.PrintTable(cmd, []store.Stats{stats}, []string{
            "Total Observations", "Total Sessions", "Total Prompts",
            "Projects", "Oldest", "Latest",
        })
        return nil
    },
}
```

No flags.

### Version command

```go
var (
    Version   = "dev"
    Commit    = "none"
    BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
    Use:   "version",
    Short: "Show version information",
    RunE: func(cmd *cobra.Command, args []string) error {
        info := map[string]string{
            "version":    Version,
            "commit":     Commit,
            "build_date": BuildDate,
        }
        if getFlagBool(cmd, "json") {
            return output.PrintJSON(cmd, info)
        }
        cmd.Printf("domain version %s (commit: %s, built: %s)\n",
            Version, Commit, BuildDate)
        return nil
    },
}
```

Flags: `--json` (local, sobreescribe el global)

### Output helpers

```go
package cli

func Print(cmd *cobra.Command, kind string, data any) {
    if jsonMode(cmd) {
        PrintJSON(cmd, data)
        return
    }
    // human-friendly text
}

func PrintTable(cmd *cobra.Command, rows any, cols []string) {
    if jsonMode(cmd) {
        PrintJSON(cmd, rows)
        return
    }
    table := tablewriter.NewWriter(cmd.OutOrStdout())
    table.SetHeader(cols)
    // populate
    table.Render()
}
```

### Project resolution

Temporary: `DOMAIN_PROJECT` env var, `--project` flag, then `os.Getwd()` basename.
Full resolution delegated to REQ-08-project-resolution.

```go
func resolveProject(flagValue string) string {
    if flagValue != "" { return flagValue }
    if env := os.Getenv("DOMAIN_PROJECT"); env != "" { return env }
    wd, _ := os.Getwd()
    return filepath.Base(wd)
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| CLI flags estilo `--flag=value` vs `--flag value` | Cobra soporta ambos; usar `--flag value` por claridad |
| Output siempre JSON con `jq` | La mayoría de los usuarios quieren texto legible; `--json` para scripting |
| `urfave/cli` en vez de cobra | Cobra es más usado en ecosistema Go; mejor integración con `--help` autogenerado |
| Prompt interactivo en save | No es CLI tradicional; `save <title> <msg>` es más pipe-friendly |
| Paginación tipo `less` | El output es corto; si se necesita, el usuario puede pipear a `less` |

## TDD plan

1. **Red:** `TestSaveCommand` — invoca save con args, espera output con "created" + id → falla
2. **Green:** Implementar saveCmd.RunE mínimo → pasa
3. **Red:** `TestSaveCommandValidation` — save sin title → espera error → falla
4. **Green:** Agregar validación cobra.ExactArgs → pasa
5. **Red:** `TestSearchCommand` — search existente → espera tabla con resultados → falla
6. **Green:** Implementar search + store.SearchObservations mock → pasa
7. **Red:** `TestDeleteSoft` / `TestDeleteHard` → fallan
8. **Green:** Implementar delete con flags → pasan
9. **Red:** `TestContextCommand` — context con proyecto → espera datos → falla
10. **Green:** Implementar context + store.GetActiveSession → pasa
11. **Red:** `TestStatsCommand` — stats → espera tabla → falla
12. **Green:** Implementar stats + store.GetStats → pasa
13. **Red:** `TestVersionCommand` — version → espera string → falla
14. **Green:** Implementar version con ldflags → pasa
15. **Red:** `TestGlobalJSONFlag` — todos los comandos con --json → espera JSON válido → falla
16. **Green:** Integrar output.Print con jsonMode check → pasa
17. **Sabotaje:** Romper DB path → save falla con error claro → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Proyecto auto-detection incorrecta | Flag `--project` explícito; env var `DOMAIN_PROJECT` |
| DB no existe al primer comando | `store.InitDB` crea DB + migraciones si no existe |
| Output tabular se desalinea con datos largos | `tablewriter` tiene wrapping automático |
| FTS5 search lento con muchas observaciones | Indice FTS5 ya optimizado; limit default 20 |
| Cobra help muy genérico | Custom templates con ejemplos de uso específicos |
