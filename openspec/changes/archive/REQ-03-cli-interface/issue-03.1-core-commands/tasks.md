# Tasks: issue-03.1-core-commands

## Backend

- [ ] **B1: Scaffold Go project + Cobra**
      ```bash
      mkdir -p cmd/domain internal/cli internal/store
      go mod init github.com/nunezlagos/memoria
      go get github.com/spf13/cobra github.com/spf13/pflag
      go get github.com/olekukonez/tablewriter
      go get github.com/fatih/color
      ```
      - `cmd/domain/main.go` con rootCmd y PersistentPreRun que inicializa DB
      - Flags globales: `--json`, `--project`, `--db-path`

- [ ] **B2: Implementar output helpers**
      ```
      internal/cli/output.go
      ```
      - `PrintJSON(cmd, data)` — `json.NewEncoder` con `SetEscapeHTML(false)`
      - `PrintTable(cmd, rows, cols)` — tablewriter con header y border básico
      - `PrintMessage(cmd, format, args...)` — color.Green para éxito, color.Red para error
      - `isJSONMode(cmd)` — checkea flag global `--json`

- [ ] **B3: Implementar project resolution**
      ```
      internal/cli/project.go
      ```
      - `resolveProject(flagValue string) string` — flag > env > cwd basename
      - `getDB(cmd.Context()) *sql.DB` — extract de context

- [ ] **B4: Implementar save command**
      ```
      internal/cli/save.go
      ```
      - `saveCmd` con flags `--type`, `--scope`, `--project`, `--topic-key`
      - Valida `cobra.ExactArgs(2)`
      - Construye `store.Observation` y llama a `AddObservation`
      - Muestra warning si hay candidates duplicados
      - Output: `"Created observation {id}"` en texto, `{"id": N}` en JSON

- [ ] **B5: Implementar search command**
      ```
      internal/cli/search.go
      ```
      - `searchCmd` con flags `--type`, `--project`, `--scope`, `--limit`
      - Valida `cobra.ExactArgs(1)`
      - Construye `store.SearchFilter` y llama a `SearchObservations`
      - Si 0 resultados: "No results found", exit 0
      - Output: tabla con ID, Title, Project, Type, Score, Created

- [ ] **B6: Implementar delete command**
      ```
      internal/cli/delete.go
      ```
      - `deleteCmd` con flag `--hard`
      - Valida `cobra.ExactArgs(1)`, parsea int64
      - Llama a `store.DeleteObservation`
      - Output: "observation {id} deleted" o "permanently deleted"
      - Error si ya eliminada o no existe

- [ ] **B7: Implementar context command**
      ```
      internal/cli/context.go
      ```
      - `contextCmd` con arg opcional `[project]` y flag `--scope`
      - Obtiene sesión activa (store.GetActiveSession)
      - Obtiene últimas 5 observaciones (store.RecentObservations)
      - Muestra: "Project: {name}", "Session: {id}", "Recent observations:" + tabla
      - Si no hay sesión activa: "No active session"

- [ ] **B8: Implementar stats command**
      ```
      internal/cli/stats.go
      ```
      - `statsCmd` sin args
      - Define `store.Stats` struct y `store.GetStats` function
      - Llama a store.GetStats
      - Output: tabla con métricas

- [ ] **B9: Implementar version command**
      ```
      internal/cli/version.go
      ```
      - `versionCmd` con flag local `--json` (overrides global)
      - Variables inyectadas via ldflags: `Version`, `Commit`, `BuildDate`
      - Output: `"domain version X.Y.Z (commit: abc123, built: 2026-06-07)"`

- [ ] **B10: Implementar store.SearchObservations**
      ```
      internal/store/search.go
      ```
      - `SearchFilter` struct con Query, Type, Project, Scope, Limit
      - FTS5 query: `SELECT ... FROM observations_fts WHERE observations_fts MATCH ?`
      - Joins contra observations para campos extra
      - Ordena por rank (bm25)
      - Limit default 20

- [ ] **B11: Implementar store.GetStats**
      ```
      internal/store/stats.go (o agrega a observations.go)
      ```
      ```go
      type Stats struct {
          TotalObservations int    `json:"total_observations"`
          TotalSessions     int    `json:"total_sessions"`
          TotalPrompts      int    `json:"total_prompts"`
          ProjectsCount     int    `json:"projects_count"`
          OldestObservation string `json:"oldest_observation"`
          LatestObservation string `json:"latest_observation"`
      }

      func GetStats(db *sql.DB) (Stats, error) {
          // COUNT(*) queries contra observations, sessions, user_prompts
          // COUNT(DISTINCT project) para projects_count
          // MIN(created_at), MAX(created_at) para fechas
      }
      ```

- [ ] **B12: Implementar store.GetActiveSession**
      ```
      internal/store/session.go (o agrega)
      ```
      ```go
      func GetActiveSession(db *sql.DB, project string) (*Session, error) {
          // SELECT ... FROM sessions WHERE project = ? AND ended_at IS NULL
          // ORDER BY created_at DESC LIMIT 1
      }
      ```

## Frontend

- [ ] N/A — CLI tool, no frontend

## Tests

- [ ] **T1: TestSaveCommand** — invoca save con args completos, verifica output "created"
- [ ] **T2: TestSaveCommandValidation** — save con 0 o 3 args, espera error de cobra
- [ ] **T3: TestSaveCommandDuplicate** — save dos observaciones iguales, verifica warning
- [ ] **T4: TestSearchCommand** — search con query existente, verifica resultados en tabla
- [ ] **T5: TestSearchCommandEmpty** — search sin resultados, verifica "No results found"
- [ ] **T6: TestSearchCommandFlags** — search con --type, --project, verifica filtro
- [ ] **T7: TestDeleteSoft** — delete por ID, verifica soft delete en DB
- [ ] **T8: TestDeleteHard** — delete --hard, verifica hard delete en DB
- [ ] **T9: TestDeleteNotFound** — delete ID inexistente, verifica error y exit 1
- [ ] **T10: TestContextCommand** — context con proyecto, verifica sesión y observaciones
- [ ] **T11: TestContextCommandNoSession** — context sin sesión activa, verifica mensaje
- [ ] **T12: TestStatsCommand** — stats con datos, verifica métricas correctas
- [ ] **T13: TestStatsCommandEmpty** — stats en DB vacía, verifica 0s
- [ ] **T14: TestVersionCommand** — version, verifica formato string
- [ ] **T15: TestVersionJSON** — version --json, verifica JSON valido
- [ ] **T16: TestGlobalJSONFlag** — save/search/delete con --json, verifica output JSON
- [ ] **T17: TestProjectResolution** — flag > env > cwd, verifica prioridad correcta
- [ ] **T18: Sabotaje** — corromper DB path, save/search/delete fallan con error claro
- [ ] **T19: Sabotaje** — eliminar tabla observations, stats no crashea (muestra 0s)

## Cierre

- [ ] `go build ./cmd/domain` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/cli/... -v -count=1` — suite completa verde
- [ ] `./domain --help` muestra todos los comandos correctamente
- [ ] `./memoria save "test" "content"` funciona en directorio cualquiera
- [ ] `./domain version` muestra version correcta
- [ ] Commit: `feat: implement core CLI commands save/search/delete/context/stats/version`
