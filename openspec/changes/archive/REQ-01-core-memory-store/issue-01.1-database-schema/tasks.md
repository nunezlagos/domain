# Tasks: issue-01.1-database-schema

## Backend

- [ ] **B1: Crear módulo Go y estructura de paquete**
      - `go mod init github.com/nunezlagos/memoria`
      - Crear `internal/store/store.go`
      - Agregar dependencia `modernc.org/sqlite`

- [ ] **B2: Definir DDL constants para todas las tablas**
      - `ddlSessions`, `ddlObservations`, `ddlUserPrompts`, `ddlSyncChunks`, `ddlMemoryRelations`, `ddlSyncApplyDeferred`
      - Índices para observations (session_id, project, normalized_hash, topic_key, type)
      - Índices para user_prompts (session_id) y memory_relations (source_id, target_id, session_id)
      - Migración `001_initial_schema` con todas las CREATE TABLE + CREATE INDEX

- [ ] **B3: Definir DDL para FTS5 virtual tables y triggers**
      - `ddlObservationsFTS` — `CREATE VIRTUAL TABLE IF NOT EXISTS observations_fts USING fts5(...)`
      - `ddlPromptsFTS` — `CREATE VIRTUAL TABLE IF NOT EXISTS prompts_fts USING fts5(...)`
      - Triggers AFTER INSERT/UPDATE/DELETE para ambas FTS tables
      - Migración `002_fts5_tables` con todo lo anterior

- [ ] **B4: Implementar InitDB(dsn string) (*sql.DB, error)**
      - Construir DSN con PRAGMAs via _pragma params:
        - `journal_mode(wal)`
        - `busy_timeout(5000)`
        - `synchronous(normal)`
        - `foreign_keys(on)`
      - `sql.Open("sqlite", dsn)` + `db.Ping()` para verificar conexión
      - Si `journal_mode=wal` falla, hacer fallback a `journal_mode=delete` con log warning

- [ ] **B5: Implementar RunMigrations(db *sql.DB) error**
      - Slice ordenado de migraciones: `{Name, DDL, Hash}`
      - Leer tabla `_migrations` (creada si no existe)
      - Aplicar migraciones pendientes en transacción individual cada una
      - Registrar en `_migrations`: version, applied_at, ddl_hash
      - Si hash del DDL no coincide con el registrado, log warning (schema drift detection)

- [ ] **B6: Agregar helper sanitizer para strings (evitar NULLs en FTS5)**
      - FTS5 no indexa NULLs; `observations.title` y otros campos deben ser `NOT NULL DEFAULT ''`
      - Helper `coalesce(v, '')` no necesario porque DDL ya tiene defaults

- [ ] **B7: Verificar que `modernc.org/sqlite` soporta FTS5**
      - Build + test rápido con `CREATE VIRTUAL TABLE ... USING fts5(...)`
      - Si no soporta, agregar fallback con `mattn/go-sqlite3` via build tag `cgo`

## Frontend

- [ ] N/A — HU puramente backend

## Tests

- [ ] **T1: TestSchemaCreation — todas las tablas existen tras migración**
      ```go
      func TestSchemaCreation(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          tables := queryTables(db)
          expected := []string{
              "sessions", "observations", "observations_fts",
              "user_prompts", "prompts_fts",
              "sync_chunks", "memory_relations", "sync_apply_deferred",
              "_migrations",
          }
          assert.Subset(t, tables, expected)
      }
      ```

- [ ] **T2: TestWALMode — PRAGMA journal_mode devuelve wal**
      ```go
      func TestWALMode(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          var mode string
          db.QueryRow("PRAGMA journal_mode").Scan(&mode)
          assert.Equal(t, "wal", mode)
      }
      ```

- [ ] **T3: TestForeignKeyEnforcement — FK violado → error**
      ```go
      func TestForeignKeyEnforcement(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          _, err := db.Exec(
              "INSERT INTO observations (session_id, content) VALUES ('nonexistent', 'test')",
          )
          assert.ErrorContains(t, err, "FOREIGN KEY")
      }
      ```

- [ ] **T4: TestMigrationIdempotency — dos ejecuciones seguidas sin error**
      ```go
      func TestMigrationIdempotency(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          err1 := RunMigrations(db)
          err2 := RunMigrations(db)
          assert.NoError(t, err1)
          assert.NoError(t, err2)
      }
      ```

- [ ] **T5: TestFTS5Tables — tablas virtuales existen**
      ```go
      func TestFTS5Tables(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          var count int
          db.QueryRow(
              "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name LIKE '%_fts'",
          ).Scan(&count)
          assert.Equal(t, 2, count) // observations_fts + prompts_fts
      }
      ```

- [ ] **T6: TestFTS5Triggers — insert en observations propaga a FTS5**
      ```go
      func TestFTS5Triggers(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          db.Exec("INSERT INTO sessions (id, project, directory) VALUES ('s1', 'p1', '/tmp')")
          db.Exec("INSERT INTO observations (session_id, title, content) VALUES ('s1', 'hello', 'world')")
          var count int
          db.QueryRow("SELECT COUNT(*) FROM observations_fts WHERE observations_fts MATCH 'hello'").Scan(&count)
          assert.Equal(t, 1, count)
      }
      ```

- [ ] **T7: TestBusyTimeout — PRAGMA busy_timeout es 5000**
      ```go
      func TestBusyTimeout(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          var timeout int
          db.QueryRow("PRAGMA busy_timeout").Scan(&timeout)
          assert.Equal(t, 5000, timeout)
      }
      ```

- [ ] **T8: TestSyncChunksCompositePK — duplicado viola unique constraint**
      ```go
      func TestSyncChunksCompositePK(t *testing.T) {
          db := setupTestDB(t)
          defer db.Close()
          db.Exec("INSERT INTO sync_chunks (target_key, chunk_id) VALUES ('k1', 'c1')")
          _, err := db.Exec("INSERT INTO sync_chunks (target_key, chunk_id) VALUES ('k1', 'c1')")
          assert.Error(t, err)
      }
      ```

- [ ] **T9: setupTestDB helper**
      ```go
      func setupTestDB(t *testing.T) *sql.DB {
          t.Helper()
          db, err := InitDB(":memory:")
          require.NoError(t, err)
          require.NoError(t, RunMigrations(db))
          return db
      }
      ```

- [ ] **T10: Sabotaje — romper FK → confirmar test cae → restaurar**
      1. En DDL de observations, cambiar `session_id TEXT NOT NULL REFERENCES sessions(id)` a `session_id TEXT NOT NULL`
      2. Ejecutar TestForeignKeyEnforcement → debe fallar (no hay FK que violar)
      3. Restaurar DDL original
      4. Ejecutar TestForeignKeyEnforcement nuevamente → debe pasar
      5. Documentar el sabotaje en el test con comentario

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/store/... -v` — suite completa verde
- [ ] Verificar que `go.mod` tiene solo `modernc.org/sqlite` como dependencia externa
- [ ] Commit: `feat: initialize SQLite schema with all tables, FTS5, and migrations`
