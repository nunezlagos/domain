# Design: HU-12.1-doctor-readonly

## Decisión arquitectónica

### Check interface

```go
type CheckResult struct {
    Name     string `json:"name"`
    Status   string `json:"status"`    // pass, fail, warn, timeout, skip
    Message  string `json:"message,omitempty"`
    Duration string `json:"duration"`
    Category string `json:"category"`
}

type Check func(ctx context.Context, db *sql.DB, cfg *config.Config) CheckResult
```

### Doctor report

```go
type DoctorReport struct {
    Timestamp  string        `json:"timestamp"`
    Version    string        `json:"version"`
    Duration   string        `json:"duration"`
    Status     string        `json:"status"` // pass, fail, warn
    Categories []CheckResult `json:"categories"`
    Summary    ReportSummary `json:"summary"`
}

type ReportSummary struct {
    Total   int `json:"total"`
    Passed  int `json:"passed"`
    Failed  int `json:"failed"`
    Warned  int `json:"warned"`
    Skipped int `json:"skipped"`
}
```

### Checks por categoría

```go
var doctorChecks = []struct{
    Name   string
    Checks []Check
}{
    {
        Name: "project",
        Checks: []Check{
            checkProjectDirs,
            checkConfigFile,
            checkGitRemote,
        },
    },
    {
        Name: "sessions",
        Checks: []Check{
            checkOpenSessions,
            checkOrphanObservations,
            checkSessionsWithoutObservations,
        },
    },
    {
        Name: "sync",
        Checks: []Check{
            checkServerReachable,
            checkTokenValid,
            checkEnrollmentActive,
            checkLastSync,
        },
    },
    {
        Name: "db",
        Checks: []Check{
            checkDBIntegrity,
            checkDiskSpace,
            checkMemoryUsage,
        },
    },
}
```

### Check implementaciones

```go
func checkDBIntegrity(ctx context.Context, db *sql.DB, cfg *config.Config) CheckResult {
    start := time.Now()
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    rows, err := db.QueryContext(ctx, "PRAGMA integrity_check")
    if err != nil {
        return CheckResult{
            Name: "db.integrity_check", Status: "fail",
            Message: err.Error(), Duration: time.Since(start).String(),
        }
    }
    defer rows.Close()

    var result string
    if rows.Next() {
        rows.Scan(&result)
    }
    if result != "ok" {
        return CheckResult{
            Name: "db.integrity_check", Status: "fail",
            Message: result, Duration: time.Since(start).String(),
        }
    }
    return CheckResult{
        Name: "db.integrity_check", Status: "pass",
        Duration: time.Since(start).String(),
    }
}

func checkOrphanObservations(ctx context.Context, db *sql.DB, cfg *config.Config) CheckResult {
    start := time.Now()
    var count int
    err := db.QueryRowContext(ctx,
        `SELECT COUNT(*) FROM observations o
         LEFT JOIN sessions s ON s.id = o.session_id
         WHERE s.id IS NULL`,
    ).Scan(&count)

    if err != nil {
        return CheckResult{Name: "sessions.orphan_observations", Status: "fail", Message: err.Error()}
    }
    if count > 0 {
        return CheckResult{Name: "sessions.orphan_observations", Status: "warn",
            Message: fmt.Sprintf("%d orphan observations found", count)}
    }
    return CheckResult{Name: "sessions.orphan_observations", Status: "pass"}
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Single check monolithic | Categorías permiten mejor organización y reporte granular |
| Sin timeouts | Un check lento (git remote) bloquearía todo el doctor |
| Output solo JSON | Table output es más legible para humanos; --json para máquinas |

## TDD plan

1. **Red:** Doctor ejecuta checks y retorna reporte → falla
2. **Green:** Implement doctor loop + CheckResult → pasa
3. **Red:** DB integrity check retorna pass/fail → falla
4. **Green:** Implement PRAGMA integrity_check → pasa
5. **Red:** Orphan observations check retorna warn → falla
6. **Green:** Implement LEFT JOIN query → pasa

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| PRAGMA integrity_check lento en DB grande | Timeout 30s para integrity_check (vs 5s otros) |
| Disk space check no portable | Usar `os.Stat` para tamaño de DB; `syscall.Statfs` para espacio libre (Unix) |
| Memory check no disponible en todos los OS | Omitir si no soportado; status "skip" con mensaje |
