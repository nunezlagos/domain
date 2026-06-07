# Tasks: HU-03.2-data-commands

## Backend

- [ ] **B1: Implementar export command handler**
      ```
      internal/cli/export.go
      ```
      - `exportCmd` con flag `--project`
      - Si no hay filename: escribe JSON a stdout
      - Si hay filename: escribe JSON a archivo (`os.WriteFile`)
      - Llama a `store.Export(db, project)` (HU-01.8)
      - Output: mensaje de confirmación o JSON directo en stdout

- [ ] **B2: Implementar import command handler**
      ```
      internal/cli/import.go
      ```
      - `importCmd` con arg obligatorio `<file>` (usa `-` para stdin)
      - Lee archivo completo o stdin
      - Llama a `store.Import(db, data)` (HU-01.8)
      - Output: "Imported from {source}: N observations, M sessions, K prompts"
      - Error si JSON inválido o datos corruptos

- [ ] **B3: Implementar projects command tree**
      ```
      internal/cli/projects.go
      ```
      - `projectsCmd` raíz con subcomandos
      - `projects listCmd` — llama a `store.ListProjects`, tabla con Project, Observations, Last Activity
      - `projects consolidateCmd` — flag `--project`, llama a `store.ConsolidateProjects`
      - `projects pruneCmd` — flag `--dry-run`, llama a `store.PruneProjects`

- [ ] **B4: Implementar store.ListProjects**
      ```
      internal/store/projects.go
      ```
      ```go
      type ProjectInfo struct {
          Name         string `json:"name"`
          Observations int    `json:"observations"`
          LastActivity string `json:"last_activity,omitempty"`
      }

      func ListProjects(db *sql.DB) ([]ProjectInfo, error) {
          query := `
              SELECT project, COUNT(*), MAX(created_at)
              FROM observations
              WHERE deleted_at IS NULL
              GROUP BY project
              ORDER BY project
          `
          rows, err := db.Query(query)
          // scan...
      }
      ```

- [ ] **B5: Implementar store.ConsolidateProjects**
      ```go
      func ConsolidateProjects(db *sql.DB, targetProject string) (int64, error) {
          if targetProject != "" {
              result, err := db.Exec(`
                  UPDATE observations
                  SET project = ?
                  WHERE LOWER(project) = LOWER(?) AND project != ?
              `, targetProject, targetProject, targetProject)
              // return rows affected
          }
          // else: auto-consolidate: agrupa por LOWER, elige el más usado como canónico
      }
      ```

- [ ] **B6: Implementar store.PruneProjects**
      ```go
      func PruneProjects(db *sql.DB, dryRun bool) ([]string, error) {
          query := `
              SELECT DISTINCT project FROM observations
              WHERE project != ''
              EXCEPT
              SELECT DISTINCT project FROM observations
              WHERE deleted_at IS NULL AND project != ''
          `
          rows, err := db.Query(query)
          // collect projects with only soft-deleted obs
          if !dryRun {
              for _, p := range projects {
                  db.Exec(`UPDATE observations SET project = '' WHERE project = ?`, p)
              }
          }
          return projects, nil
      }
      ```

- [ ] **B7: Registrar subcomandos en rootCmd**
      ```go
      func init() {
          rootCmd.AddCommand(saveCmd, searchCmd, deleteCmd, contextCmd, statsCmd, versionCmd)
          rootCmd.AddCommand(exportCmd, importCmd, projectsCmd)
          projectsCmd.AddCommand(projectsListCmd, projectsConsolidateCmd, projectsPruneCmd)
      }
      ```

## Frontend

- [ ] N/A — CLI tool

## Tests

- [ ] **T1: TestExportCommandStdout** — export sin archivo, verifica JSON en stdout
- [ ] **T2: TestExportCommandFile** — export a archivo temporal, verifica contenido y mensaje
- [ ] **T3: TestExportCommandProjectFilter** — export --project, verifica solo ese proyecto en JSON
- [ ] **T4: TestExportCommandEmpty** — DB vacía, verifica arrays vacíos en JSON
- [ ] **T5: TestExportCommandInvalidPath** — export a ruta sin permiso, verifica error
- [ ] **T6: TestImportCommandFile** — import desde archivo JSON, verifica datos en DB
- [ ] **T7: TestImportCommandStdin** — import desde stdin, pipea JSON, verifica datos
- [ ] **T8: TestImportCommandInvalidJSON** — import archivo corrupto, verifica error "invalid JSON"
- [ ] **T9: TestImportCommandMissingFields** — import sin campo "observations", verifica error
- [ ] **T10: TestImportCommandDuplicateSession** — import con sesión existente, verifica INSERT OR IGNORE
- [ ] **T11: TestImportCommandTransactional** — JSON con error a medio camino, DB no modificada
- [ ] **T12: TestProjectsList** — list con proyectos, verifica tabla correcta
- [ ] **T13: TestProjectsListEmpty** — list sin proyectos, verifica "No projects found"
- [ ] **T14: TestProjectsConsolidate** — consolidate "MEMORIA"+"Domain"+"Domain" → 1 proyecto
- [ ] **T15: TestProjectsConsolidateWithFlag** — consolidate --project "Domain", verifica merge
- [ ] **T16: TestProjectsPrune** — create proyecto sin obs activas, prune, verifica limpieza
- [ ] **T17: TestProjectsPruneDryRun** — prune --dry-run, verifica no cambios
- [ ] **T18: TestProjectsPruneNone** — todos con obs, prune → "No empty projects to prune"
- [ ] **T19: Sabotaje** — Import JSON con FK violation → rollback total → DB unchanged

## Cierre

- [ ] `go build ./cmd/domain` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/cli/... -v -count=1` — suite completa verde
- [ ] `./memoria export test.json` funciona y produce JSON válido
- [ ] `./memoria import test.json` restaura datos correctamente
- [ ] `./memoria projects list | consolidate | prune` funcionan
- [ ] Commit: `feat: implement export/import/projects CLI commands`
