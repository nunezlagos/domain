# Design: HU-05.6-http-stats-doctor

## Decisión arquitectónica

### StatsRepo interface

```go
type StatsRepo interface {
    GetStats(ctx context.Context) (Stats, error)
    RunDoctor(ctx context.Context, project, check string) ([]DoctorCheck, error)
    Health(ctx context.Context) (Health, error)
}
```

### Stats queries

```go
func (r *statsRepo) GetStats(ctx context.Context) (Stats, error) {
    var s Stats

    r.db.QueryRowContext(ctx,
        `SELECT COUNT(*) FROM observations WHERE deleted_at IS NULL`).Scan(&s.TotalObservations)
    r.db.QueryRowContext(ctx,
        `SELECT COUNT(*) FROM sessions`).Scan(&s.TotalSessions)
    r.db.QueryRowContext(ctx,
        `SELECT COUNT(*) FROM user_prompts`).Scan(&s.TotalPrompts)
    r.db.QueryRowContext(ctx,
        `SELECT COUNT(DISTINCT project) FROM observations`).Scan(&s.TotalProjects)

    // DB file size
    var dbPath string
    r.db.QueryRow("PRAGMA database_list").Scan(&dbPath, &dbPath) // second is file
    if fi, err := os.Stat(dbPath); err == nil {
        s.DBSizeBytes = fi.Size()
    }
    s.DBPath = dbPath

    r.db.QueryRowContext(ctx,
        `SELECT MIN(created_at) FROM observations WHERE deleted_at IS NULL`).Scan(&s.OldestObservation)
    r.db.QueryRowContext(ctx,
        `SELECT MAX(created_at) FROM observations WHERE deleted_at IS NULL`).Scan(&s.NewestObservation)

    return s, nil
}
```

### Doctor check registry pattern

```go
type DoctorFn func(ctx context.Context, db *sql.DB, project string) DoctorCheck

var doctorChecks = map[string]DoctorFn{
    "orphans":       checkOrphanObservations,
    "fts5":          checkFTS5Integrity,
    "schema":        checkSchemaVersion,
    "missing_index": checkMissingIndexes,
    "wal_mode":      checkWALMode,
}

func checkOrphanObservations(ctx context.Context, db *sql.DB, project string) DoctorCheck {
    q := `SELECT COUNT(*) FROM observations o
          LEFT JOIN sessions s ON o.session_id = s.id
          WHERE s.id IS NULL AND o.deleted_at IS NULL`
    var count int
    db.QueryRowContext(ctx, q).Scan(&count)
    if count > 0 {
        return DoctorCheck{"orphan_observations", "warn",
            fmt.Sprintf("found %d observations without valid session", count)}
    }
    return DoctorCheck{"orphan_observations", "pass", "no orphan observations"}
}

func checkFTS5Integrity(ctx context.Context, db *sql.DB, project string) DoctorCheck {
    var count int
    err := db.QueryRowContext(ctx,
        `SELECT COUNT(*) FROM observations_fts`).Scan(&count)
    if err != nil {
        return DoctorCheck{"fts5_index", "fail", fmt.Sprintf("FTS5 error: %v", err)}
    }
    return DoctorCheck{"fts5_index", "pass",
        fmt.Sprintf("FTS5 index healthy, %d entries", count)}
}
```

### Health check

```go
var startTime = time.Now()

func (r *statsRepo) Health(ctx context.Context) (Health, error) {
    h := Health{
        Version: version.String(),
        Uptime:  time.Since(startTime).Round(time.Second).String(),
    }

    if err := r.db.PingContext(ctx); err != nil {
        h.Status = "degraded"
        h.DBAlive = false
        return h, nil
    }

    h.Status = "ok"
    h.DBAlive = true
    return h, nil
}
```

### Route registration

```go
func RegisterStatsRoutes(mux *http.ServeMux, repo StatsRepo) {
    mux.HandleFunc("GET /stats", handleStats(repo))
    mux.HandleFunc("GET /doctor", handleDoctor(repo))
    mux.HandleFunc("GET /health", handleHealth(repo))
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| /health con autenticación | Health debe ser accesible sin auth para load balancers |
| Doctor checks asíncronos | Son rápidos (< 1s); async agrega complejidad innecesaria |
| Prometheus metrics endpoint | Overkill para DB local; stats simple es suficiente |

## Diagrama

```
Client HTTP                          memoria server
    |                                       |
    | GET /stats                             |
    |   +-- COUNT queries -----------------> observations/sessions/prompts
    |   +-- os.Stat DB file                  |
    |                                       |
    | GET /doctor[?project=&check=]          |
    |   +-- orphan check ------------------> LEFT JOIN sessions
    |   +-- FTS5 check --------------------> SELECT FROM observations_fts
    |   +-- schema check ------------------> SELECT FROM _migrations
    |   +-- WAL check ---------------------> PRAGMA journal_mode
    |                                       |
    | GET /health                            |
    |   +-- db.Ping()                        |
    |   +-- version + uptime                 |
    |                                       |
    +--------> api/stats.go --------------> store/stats.go
                                                |
                                            SQLite DB
```

## TDD plan

1. **Red:** Test GET /stats → 200, campos → falla
2. **Green:** Stats handler → pasa
3. **Red:** Test GET /doctor → array de checks → falla
4. **Green:** Doctor handler con checks registrados → pasa
5. **Red:** Test GET /doctor?check=orphans → solo ese check → falla
6. **Green:** Filtrar por check name → pasa
7. **Red:** Test GET /health → 200, status ok → falla
8. **Green:** Health handler → pasa
9. **Sabotaje:** Cerrar DB intencionalmente → /health retorna degraded → test cae → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| os.Stat falla en DB path virtual | PRAGMA database_list da el path real; si falla, db_size=0 |
| Doctor check no encontrado | Si ?check= no coincide, retornar 400 con lista de checks válidos |
| Health dependiente de variable version | Usar debug.ReadBuildInfo como fallback si no hay ldflags |
