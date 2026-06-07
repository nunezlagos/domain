# Design: HU-03.2-data-commands

## Decisión arquitectónica

### Export command

```go
var exportCmd = &cobra.Command{
    Use:   "export [file]",
    Short: "Export all memory data to JSON",
    Args:  cobra.MaximumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        project, _ := cmd.Flags().GetString("project")
        data, err := store.Export(getDB(cmd), project)
        if err != nil { return err }

        if len(args) == 0 {
            // stdout
            cmd.Println(string(data))
            return nil
        }
        filename := args[0]
        if err := os.WriteFile(filename, data, 0644); err != nil {
            return fmt.Errorf("cannot write file: %w", err)
        }
        output.PrintMessage(cmd, "Exported to %s", filename)
        return nil
    },
}
```

Flags: `--project`

### Import command

```go
var importCmd = &cobra.Command{
    Use:   "import <file>",
    Short: "Import memory data from JSON file",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        var data []byte
        var source string
        if args[0] == "-" {
            data, _ = io.ReadAll(os.Stdin)
            source = "stdin"
        } else {
            data, _ = os.ReadFile(args[0])
            source = args[0]
        }

        result, err := store.Import(getDB(cmd), data)
        if err != nil { return err }

        output.PrintMessage(cmd,
            "Imported from %s: %d observations, %d sessions, %d prompts",
            source, result.Observations, result.Sessions, result.Prompts)
        return nil
    },
}
```

No flags. `-` para stdin.

### Projects commands

```go
var projectsCmd = &cobra.Command{
    Use:   "projects {list|consolidate|prune}",
    Short: "Manage memory projects",
}

var projectsListCmd = &cobra.Command{
    Use:   "list",
    Short: "List all projects with observation counts",
    RunE: func(cmd *cobra.Command, args []string) error {
        projects, err := store.ListProjects(getDB(cmd))
        if err != nil { return err }
        if len(projects) == 0 {
            cmd.Println("No projects found")
            return nil
        }
        output.PrintTable(cmd, projects, []string{"Project", "Observations", "Last Activity"})
        return nil
    },
}

var projectsConsolidateCmd = &cobra.Command{
    Use:   "consolidate",
    Short: "Consolidate duplicate project names (case-insensitive merge)",
    RunE: func(cmd *cobra.Command, args []string) error {
        project, _ := cmd.Flags().GetString("project")
        count, err := store.ConsolidateProjects(getDB(cmd), project)
        if err != nil { return err }
        output.PrintMessage(cmd, "Consolidated %d observations", count)
        return nil
    },
}

var projectsPruneCmd = &cobra.Command{
    Use:   "prune",
    Short: "Remove empty projects (no active observations)",
    RunE: func(cmd *cobra.Command, args []string) error {
        dryRun, _ := cmd.Flags().GetBool("dry-run")
        pruned, err := store.PruneProjects(getDB(cmd), dryRun)
        if err != nil { return err }
        if len(pruned) == 0 {
            cmd.Println("No empty projects to prune")
            return nil
        }
        if dryRun {
            output.PrintTable(cmd, pruned, []string{"Project", "Status"})
            return nil
        }
        output.PrintMessage(cmd, "Pruned %d empty projects", len(pruned))
        return nil
    },
}
```

Flags: consolidate: `--project`; prune: `--dry-run`

### Store layer additions

```go
// internal/store/projects.go

type ProjectInfo struct {
    Name         string `json:"name"`
    Observations int    `json:"observations"`
    LastActivity string `json:"last_activity,omitempty"`
}

func ListProjects(db *sql.DB) ([]ProjectInfo, error) {
    rows, err := db.Query(`
        SELECT project, COUNT(*), MAX(created_at)
        FROM observations
        WHERE deleted_at IS NULL
        GROUP BY project
        ORDER BY project
    `)
    // scan into []ProjectInfo
}

func ConsolidateProjects(db *sql.DB, targetProject string) (int, error) {
    // If targetProject set: UPDATE observations SET project = ? WHERE LOWER(project) = LOWER(?)
    // Else: aggregate all LOWER(project) variants into canonical (most used) name
}

func PruneProjects(db *sql.DB, dryRun bool) ([]string, error) {
    // SELECT DISTINCT project FROM observations WHERE deleted_at IS NULL
    // vs SELECT DISTINCT project FROM observations
    // difference = projects with only soft-deleted obs → safe to prune
    // If not dryRun: UPDATE observations SET project = '' WHERE project IN (...)
}
```

### Export/Import format

```go
type ExportData struct {
    Version      int                `json:"version"`
    ExportedAt   string             `json:"exported_at"`
    Sessions     []store.Session    `json:"sessions"`
    Observations []store.Observation `json:"observations"`
    Prompts      []store.UserPrompt `json:"prompts"`
}
```

Version actual: 1. `ExportedAt` en ISO 8601.

### Import flow

```
1. Parsear JSON a ExportData
2. Validar estructura: version >= 1, fields no nil
3. Iniciar transacción
4. INSERT OR IGNORE en sessions
5. INSERT en observations (con normalized_hash recalculation)
6. INSERT en user_prompts
7. Commit
8. Retornar conteo de cada tipo
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| JSON streaming (json.Decoder) | Volumen esperado <50MB; simplicidad de json.Marshal vale la pena |
| ZIP compress en export | Comprimir agrega complejidad; usuario puede pipear a gzip |
| YAML como alternativa de formato | YAML es menos estándar que JSON para interoperabilidad |
| Projects como subcomando de `memoria project` (singular) | Plural es más natural para list/consolidate/prune |
| Auto-consolidate en cada save | Operación pesada innecesaria; mejor bajo demanda |

## TDD plan

1. **Red:** `TestExportCommand` — export con datos, verifica JSON output en stdout → falla
2. **Green:** Implementar exportCmd → pasa
3. **Red:** `TestExportToFile` — export a archivo, verifica archivo existe y contenido → falla
4. **Green:** Implementar file write → pasa
5. **Red:** `TestExportProjectFilter` — export --project → verifica filtro → falla
6. **Green:** delegate a store.Export(project) → pasa
7. **Red:** `TestImportCommand` — import JSON válido, verifica entidades en DB → falla
8. **Green:** Implementar importCmd → pasa
9. **Red:** `TestImportStdin` — import -, pipea JSON → verifica import → falla
10. **Green:** Manejar "-" como stdin → pasa
11. **Red:** `TestImportInvalidJSON` → error → falla
12. **Green:** Validación de JSON → pasa
13. **Red:** `TestImportTransactional` — JSON con error a medio camino → DB no modificada → falla
14. **Green:** Transacción + validación previa → pasa
15. **Red:** `TestProjectsList` / `TestProjectsConsolidate` / `TestProjectsPrune` → fallan
16. **Green:** Implementar store.ListProjects, ConsolidateProjects, PruneProjects → pasan
17. **Red:** `TestProjectsPruneDryRun` → verifica no-modificación → falla
18. **Green:** dry-run flag → pasa
19. **Sabotaje:** Import con FK violation → rollback → DB intacta

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Export archivos grandes causan OOM | Usar json.Marshal + os.WriteFile; si es problema futuro, migrar a streaming |
| Import valida estructura pero no datos referenciales | Validación de FK al insertar; si falla, rollback |
| Consolidate renombra a LOWER() que puede no ser el nombre canónico deseado | El usuario puede especificar --project para controlar el nombre final |
| Prune sin --dry-run elimina metadata | Output claro de qué se eliminó; las observaciones no se pierden (solo cambian de proyecto) |
