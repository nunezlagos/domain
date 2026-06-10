# Design: issue-03.3-admin-commands

## Decisión arquitectónica

### Doctor command

```
internal/cli/doctor.go
└── doctorCmd
    └── checks[] = {
        {"database_exists", checkDatabaseExists},
        {"migrations_applied", checkMigrations},
        {"fts5_index", checkFTS5Index},
        {"disk_space", checkDiskSpace},
        {"file_permissions", checkPermissions},
    }
```

```go
type CheckResult struct {
    Name   string `json:"name"`
    Status string `json:"status"` // "pass", "fail", "warn"
    Detail string `json:"detail,omitempty"`
}

var doctorCmd = &cobra.Command{
    Use:   "doctor",
    Short: "Run system diagnostics",
    RunE: func(cmd *cobra.Command, args []string) error {
        checkName, _ := cmd.Flags().GetString("check")
        project, _ := cmd.Flags().GetString("project")

        results := runChecks(getDB(cmd), checkName, project)
        failed := false
        for _, r := range results {
            if r.Status == "fail" { failed = true }
        }

        if jsonMode(cmd) {
            return output.PrintJSON(cmd, results)
        }

        output.PrintTable(cmd, results, []string{"Check", "Status", "Detail"})
        if failed { return fmt.Errorf("some checks failed") }
        return nil
    },
}
```

Flags: `--project`, `--check` (single check name), `--json`

### Conflicts command tree

```go
var conflictsCmd = &cobra.Command{Use: "conflicts", Short: "Manage observation conflicts"}
// subcommands:
//   list    — lista todos los conflictos
//   show <id> — detalle de un conflicto
//   stats   — estadísticas de conflictos
//   scan    — escanea nuevos conflictos
//   deferred — lista conflictos diferidos
```

Cada subcomando llama a funciones de `internal/store/conflicts.go` que serán implementadas completamente en REQ-10. Por ahora, implementamos stubs que retornan datos mock o "not implemented" para funcionalidades complejas.

```go
var conflictsScanCmd = &cobra.Command{
    Use:   "scan",
    Short: "Scan for new conflicts (lexical + semantic)",
    RunE: func(cmd *cobra.Command, args []string) error {
        // stub: delegar a store.ScanConflicts cuando REQ-10 esté lista
        return errors.New("conflict scanning not available until REQ-10 is implemented")
    },
}
```

Flags por subcomando:
- `list`: `--project`, `--type` (lexical|semantic)
- `scan`: `--project`, `--strategy` (lexical|semantic|both)

### Cloud command tree

```go
var cloudCmd = &cobra.Command{Use: "cloud", Short: "Manage cloud sync configuration"}

// cloud config — muestra configuración actual (sin token)
// cloud status — test de conexión + estado
// cloud enroll — setup inicial: endpoint + token
// cloud serve — inicia servidor HTTP local
// cloud upgrade — migra schema cloud
```

```go
type CloudConfig struct {
    Endpoint   string `json:"endpoint"`
    SyncEnabled bool  `json:"sync_enabled"`
    LastSync   string `json:"last_sync,omitempty"`
}

var cloudEnrollCmd = &cobra.Command{
    Use:   "enroll",
    Short: "Enroll with a cloud endpoint",
    RunE: func(cmd *cobra.Command, args []string) error {
        endpoint, _ := cmd.Flags().GetString("endpoint")
        token, _ := cmd.Flags().GetString("token")
        if endpoint == "" || token == "" {
            return fmt.Errorf("--endpoint and --token are required")
        }
        // Guardar config en ~/.memoria/cloud.json
        cfg := CloudConfig{Endpoint: endpoint, SyncEnabled: true}
        if err := saveCloudConfig(cfg, token); err != nil { return err }
        // Test connection
        if err := testCloudConnection(endpoint, token); err != nil {
            return fmt.Errorf("enrollment saved but connection failed: %w", err)
        }
        cmd.Println("Enrolled successfully")
        return nil
    },
}

var cloudServeCmd = &cobra.Command{
    Use:   "serve",
    Short: "Start cloud HTTP server for sync",
    RunE: func(cmd *cobra.Command, args []string) error {
        port, _ := cmd.Flags().GetInt("port")
        if port == 0 { port = 8080 }
        cmd.Printf("Serving cloud API on :%d\n", port)
        // stub: en REQ-05/REQ-09 se implementa el servidor real
        return fmt.Errorf("cloud serve not available until REQ-05/REQ-09")
    },
}
```

### Sync command

```go
var syncCmd = &cobra.Command{
    Use:   "sync",
    Short: "Synchronize memory data",
    RunE: func(cmd *cobra.Command, args []string) error {
        doCloud, _ := cmd.Flags().GetBool("cloud")
        doImport, _ := cmd.Flags().GetBool("import")
        doStatus, _ := cmd.Flags().GetBool("status")
        doAll, _ := cmd.Flags().GetBool("all")
        project, _ := cmd.Flags().GetString("project")

        if doAll {
            // run cloud sync + import
            return syncAll(cmd, project)
        }
        if doStatus {
            return syncStatus(cmd, project)
        }
        if doCloud {
            return syncCloud(cmd, project)
        }
        if doImport {
            return syncImport(cmd)
        }
        return cmd.Help()
    },
}
```

Flags: `--import`, `--status`, `--cloud`, `--project`, `--all`

### Store stubs pattern

Para funcionalidades que dependen de otras REQs, definimos las funciones en el store layer con implementación stub:

```go
// internal/store/conflicts.go (stub hasta REQ-10)
func ScanConflicts(db *sql.DB, project string) (int, error) {
    return 0, fmt.Errorf("not implemented: depends on REQ-10-conflict-detection")
}
```

Esto permite que los CLI handlers estén completos y compilen, con errores claros cuando se invoca funcionalidad no implementada.

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Doctor como script externo | Debe ser parte del mismo binary para consistencia |
| Conflicts en store layer sin CLI | El usuario necesita visibilidad de conflictos |
| Cloud config en DB en vez de file | Archivo separado es más seguro (token 0600) y no depende de DB |
| Sync automático cada N minutos | Será feature futuro; por ahora es under-demand via CLI |

## TDD plan

1. **Red:** `TestDoctorCommand` — doctor en DB saludable, todos pass → falla
2. **Green:** Implementar doctor con checks básicos → pasa
3. **Red:** `TestDoctorCommandFail` — DB sin migraciones → check fail → falla
4. **Green:** checkMigrations detecta tabla faltante → pasa
5. **Red:** `TestDoctorCheckSpecific` — --check database_exists → solo ese check → falla
6. **Green:** Filtro por check name → pasa
7. **Red:** `TestDoctorJSON` — --json → output JSON → falla
8. **Green:** jsonMode en doctor → pasa
9. **Red:** `TestConflictsList` / `TestConflictsShow` / `TestConflictsStats` → fallan
10. **Green:** Conflicts list con datos mock / store stubs → pasan
11. **Red:** `TestConflictsScan` → stub → "not implemented" → falla (espera error)
12. **Green:** Confirmar mensaje claro → pasa
13. **Red:** `TestCloudConfig` / `TestCloudEnroll` / `TestCloudStatus` → fallan
14. **Green:** Cloud config file I/O → pasan
15. **Red:** `TestSyncStatus` / `TestSyncCloud` / `TestSyncImport` → fallan
16. **Green:** Sync handlers con stubs → pasan
17. **Sabotaje:** Doctor sin DB → checks fallan gracefulmente, no crash

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Stubs quedan como "not implemented" indefinidamente | Cada stub tiene issue claro referenciando la REQ dependiente |
| Cloud token en plaintext | Archivo separado con permiso 0600; nunca se muestra en output |
| Doctor lento con muchas observaciones | Checks son queries simples (COUNT, EXISTS); disk space es stat() |
| conflicts scan pesado en DB grande | Flag `--project` para acotar; el CLI advierte si no hay filtro |
