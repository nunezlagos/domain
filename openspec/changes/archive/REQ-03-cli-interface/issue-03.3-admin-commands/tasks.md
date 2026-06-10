# Tasks: issue-03.3-admin-commands

## Backend

- [ ] **B1: Implementar doctor command**
      ```
      internal/cli/doctor.go
      internal/store/diagnostics.go
      ```
      - `doctorCmd` con flags `--project`, `--check`, `--json`
      - Checks array: database_exists, migrations_applied, fts5_index, disk_space, file_permissions
      - Cada check es una función `func(db *sql.DB, project string) CheckResult`
      - Si `--check` está set, filtrar a ese check
      - Si `--json`, output JSON array
      - Exit code 1 si algún check es "fail"

- [ ] **B2: Implementar database_exists check**
      ```go
      func checkDatabaseExists(db *sql.DB) CheckResult {
          var name string
          err := db.QueryRow("PRAGMA database_list").Scan(&name, &name, &name)
          if err != nil { return CheckResult{"database_exists", "fail", err.Error()} }
          return CheckResult{"database_exists", "pass", "database accessible"}
      }
      ```

- [ ] **B3: Implementar migrations_applied check**
      ```go
      func checkMigrations(db *sql.DB) CheckResult {
          rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name='schema_migrations'")
          if err != nil || !rows.Next() {
              return CheckResult{"migrations_applied", "fail", "schema_migrations table not found"}
          }
          // verificar que todas las migraciones están aplicadas
          var count int
          db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE applied_at IS NOT NULL").Scan(&count)
          // ...
      }
      ```

- [ ] **B4: Implementar fts5_index check**
      ```go
      func checkFTS5Index(db *sql.DB) CheckResult {
          var name string
          err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='observations_fts'").Scan(&name)
          if err != nil { return CheckResult{"fts5_index", "fail", "FTS5 table not found"} }
          // check row count match between observations and observations_fts
          return CheckResult{"fts5_index", "pass", "FTS5 index OK"}
      }
      ```

- [ ] **B5: Implementar disk_space y file_permissions checks**
      ```go
      func checkDiskSpace(db *sql.DB) CheckResult {
          // usar syscall.Statfs en Linux para verificar espacio disponible
          // warn si < 100MB, fail si < 10MB
      }

      func checkPermissions(db *sql.DB) CheckResult {
          // verificar que el archivo DB tiene permisos 0600
          // verificar que el directorio ~/.memoria tiene permisos 0700
      }
      ```

- [ ] **B6: Implementar conflicts command tree**
      ```
      internal/cli/conflicts.go
      internal/store/conflicts.go (stub)
      ```
      - `conflictsCmd` con subcomandos list, show, stats, scan, deferred
      - `list`: query observaciones con mismo normalized_hash, agrupar por hash
      - `show <id>`: detalle de observación + sus similares
      - `stats`: COUNT(*) agrupado por tipo/similaridad
      - `scan`: stub → "not available until REQ-10"
      - `deferred`: lista conflictos con status "deferred" (requiere tabla conflicts en REQ-10)

- [ ] **B7: Implementar store stubs para conflictos**
      ```go
      // internal/store/conflicts.go
      func ScanConflicts(db *sql.DB, project string) (int, error) {
          return 0, fmt.Errorf("conflict scanning not available until REQ-10")
      }
      func GetConflicts(db *sql.DB, project string) ([]Conflict, error) {
          // query basica sobre observations agrupando por normalized_hash
      }
      type Conflict struct {
          ID          int64  `json:"id"`
          Title       string `json:"title"`
          SimilarID   int64  `json:"similar_id"`
          SimilarTitle string `json:"similar_title"`
          Score       float64 `json:"score"`
          Status      string `json:"status"` // "open", "resolved", "deferred"
      }
      ```

- [ ] **B8: Implementar cloud command tree**
      ```
      internal/cli/cloud.go
      ```
      - `cloudCmd` con subcomandos config, status, enroll, serve, upgrade
      - `config`: leer `~/.memoria/cloud.json`, mostrar (sin token)
      - `status`: test conexión contra endpoint, mostrar estado
      - `enroll`: flags `--endpoint`, `--token`; guardar config; test connection
      - `serve`: flag `--port`; stub hasta REQ-05/REQ-09
      - `upgrade`: stub para migraciones cloud

- [ ] **B9: Implementar cloud config file I/O**
      ```go
      // internal/cloud/config.go
      type Config struct {
          Endpoint     string `json:"endpoint"`
          SyncEnabled  bool   `json:"sync_enabled"`
          LastSync     string `json:"last_sync,omitempty"`
      }
      func LoadConfig() (*Config, string, error) // returns config + token
      func SaveConfig(cfg *Config, token string) error
      func ConfigPath() string // ~/.memoria/cloud.json
      func TokenPath() string  // ~/.memoria/cloud.token
      ```

- [ ] **B10: Implementar sync command**
      ```
      internal/cli/sync.go
      internal/sync/sync.go (stub)
      ```
      - `syncCmd` con flags `--import`, `--status`, `--cloud`, `--project`, `--all`
      - `--status`: muestra pending_local, pending_remote, last_sync
      - `--cloud`: sincroniza con cloud (stub hasta REQ-09)
      - `--import`: importa archivo de sync (stub)
      - `--all`: ejecuta cloud + import secuencialmente
      - Error claro si cloud no configurado y se requiere

- [ ] **B11: Registrar subcomandos en rootCmd**
      ```go
      func init() {
          // existing commands...
          rootCmd.AddCommand(doctorCmd, conflictsCmd, cloudCmd, syncCmd)
          conflictsCmd.AddCommand(conflictsListCmd, conflictsShowCmd,
              conflictsStatsCmd, conflictsScanCmd, conflictsDeferredCmd)
          cloudCmd.AddCommand(cloudConfigCmd, cloudStatusCmd,
              cloudEnrollCmd, cloudServeCmd, cloudUpgradeCmd)
      }
      ```

## Frontend

- [ ] N/A — CLI tool

## Tests

- [ ] **T1: TestDoctorAllPass** — DB saludable, todos los checks pass
- [ ] **T2: TestDoctorMigrationFail** — DB sin migraciones, check fail
- [ ] **T3: TestDoctorSpecificCheck** — --check database_exists, solo ese check
- [ ] **T4: TestDoctorJSON** — --json, output JSON válido
- [ ] **T5: TestDoctorNoDB** — sin DB, checks fallan gracefulmente
- [ ] **T6: TestConflictsList** — conflictos existentes, lista correcta
- [ ] **T7: TestConflictsListEmpty** — sin conflictos, "no conflicts found"
- [ ] **T8: TestConflictsShow** — show 42, detalle del conflicto
- [ ] **T9: TestConflictsStats** — estadísticas de conflictos
- [ ] **T10: TestConflictsScan** — scan, verifica stub message
- [ ] **T11: TestConflictsDeferred** — deferred list, verifica filtro
- [ ] **T12: TestCloudConfig** — config existente, muestra datos (sin token)
- [ ] **T13: TestCloudConfigNotEnrolled** — sin config, "not enrolled"
- [ ] **T14: TestCloudEnroll** — enroll con flags, verifica archivo creado
- [ ] **T15: TestCloudEnrollMissingFlags** — sin --endpoint o --token, error
- [ ] **T16: TestCloudServe** — serve --port, verifica stub message
- [ ] **T17: TestCloudStatus** — status, verifica conexión test
- [ ] **T18: TestSyncStatus** --status, verifica output
- [ ] **T19: TestSyncCloud** --cloud sin config, error claro
- [ ] **T20: TestSyncCloudWithConfig** --cloud con config, verifica sync call
- [ ] **T21: TestSyncImport** --import, verifica import call
- [ ] **T22: TestSyncAll** --all, verifica secuencia cloud+import
- [ ] **T23: TestSyncProject** --cloud --project, verifica filtro
- [ ] **T24: Sabotaje** — doctor sin permisos de DB, fail graceful

## Cierre

- [ ] `go build ./cmd/domain` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/cli/... -v -count=1` — suite completa verde
- [ ] `./memoria doctor` muestra checks
- [ ] `./memoria conflicts --help` muestra subcomandos
- [ ] `./memoria cloud --help` muestra subcomandos
- [ ] `./memoria sync --help` muestra flags
- [ ] Commit: `feat: implement admin CLI commands doctor/conflicts/cloud/sync`
