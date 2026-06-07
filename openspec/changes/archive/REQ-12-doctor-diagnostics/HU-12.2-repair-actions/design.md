# Design: HU-12.2-repair-actions

## Decisión arquitectónica

### RepairAction

```go
type RepairAction struct {
    ID          string `json:"id"`
    Description string `json:"description"`
    Category    string `json:"category"` // filesystem, db, config
    Execute     func(ctx context.Context, plan *RepairPlan) error
    Validate    func(ctx context.Context, plan *RepairPlan) bool // pre-condition check
    Prerequisite string `json:"prerequisite,omitempty"` // action ID that must run first
}

type RepairPlan struct {
    Actions      []RepairAction `json:"actions"`
    DryRun       bool           `json:"dry_run"`
    MaxActions   int            `json:"max_actions"`
    DB           *sql.DB
}
```

### Repair report

```go
type RepairReport struct {
    DryRun        bool                `json:"dry_run"`
    ActionsTaken  []RepairResult      `json:"actions_taken"`
    ActionsFailed []RepairResult      `json:"actions_failed"`
    Remaining     int                 `json:"remaining,omitempty"`
    Duration      string              `json:"duration"`
}

type RepairResult struct {
    ID          string `json:"id"`
    Description string `json:"description"`
    Status      string `json:"status"` // success, failed, skipped
    Error       string `json:"error,omitempty"`
}
```

### Repair actions

```go
// 1. Create missing session directories
func repairMissingDirs(ctx context.Context, plan *RepairPlan) error {
    rows, _ := plan.DB.QueryContext(ctx,
        "SELECT DISTINCT directory FROM sessions WHERE directory != ''")
    defer rows.Close()

    var dirs []string
    for rows.Next() {
        var dir string
        rows.Scan(&dir)
        if _, err := os.Stat(dir); os.IsNotExist(err) {
            dirs = append(dirs, dir)
        }
    }

    for _, dir := range dirs {
        if err := os.MkdirAll(dir, 0755); err != nil {
            return fmt.Errorf("failed to create %s: %w", dir, err)
        }
    }
    return nil
}

// 2. Normalize project names
func repairNormalizeProjects(ctx context.Context, plan *RepairPlan) error {
    // Find non-normalized projects
    rows, _ := plan.DB.QueryContext(ctx,
        `SELECT DISTINCT project FROM observations
         WHERE project != NormalizeProject(project)`)
    // ... actualizar cada proyecto

    // Update sessions too
    _, err := plan.DB.ExecContext(ctx,
        `UPDATE sessions SET project = NormalizeProject(project)
         WHERE project != NormalizeProject(project)`)
    return err
}

// 3. Close stale sessions (active > 48h)
func repairCloseStaleSessions(ctx context.Context, plan *RepairPlan) error {
    _, err := plan.DB.ExecContext(ctx,
        `UPDATE sessions SET
         ended_at = datetime('now'),
         status = 'closed',
         summary = COALESCE(summary, '') || ' [auto-closed by doctor repair]'
         WHERE status = 'active'
         AND started_at < datetime('now', '-48 hours')`)
    return err
}

// 4. Soft-delete orphan observations (only with --fix-orphans)
func repairOrphanObservations(ctx context.Context, plan *RepairPlan) error {
    _, err := plan.DB.ExecContext(ctx,
        `UPDATE observations SET deleted_at = datetime('now')
         WHERE id IN (
             SELECT o.id FROM observations o
             LEFT JOIN sessions s ON s.id = o.session_id
             WHERE s.id IS NULL AND o.deleted_at IS NULL
         )`)
    return err
}
```

### Repair plan generation

```go
func generateRepairPlan(ctx context.Context, db *sql.DB, opts RepairOpts) (*RepairPlan, error) {
    plan := &RepairPlan{DryRun: opts.DryRun, MaxActions: opts.MaxActions, DB: db}

    // Check each condition and add actions
    if hasMissingDirs(db) {
        plan.Actions = append(plan.Actions, RepairAction{
            ID: "fix-missing-dirs", Category: "filesystem",
            Description: "Create missing session directories",
            Execute: repairMissingDirs,
        })
    }
    if hasNonNormalizedProjects(db) {
        plan.Actions = append(plan.Actions, RepairAction{
            ID: "fix-project-normalization", Category: "db",
            Description: "Normalize project names",
            Execute: repairNormalizeProjects,
        })
    }
    if hasStaleSessions(db) {
        plan.Actions = append(plan.Actions, RepairAction{
            ID: "fix-stale-sessions", Category: "db",
            Description: "Close sessions active > 48h",
            Execute: repairCloseStaleSessions,
        })
    }
    if opts.FixOrphans && hasOrphanObservations(db) {
        plan.Actions = append(plan.Actions, RepairAction{
            ID: "fix-orphan-observations", Category: "db",
            Description: "Soft-delete orphan observations",
            Execute: repairOrphanObservations,
        })
    }

    // Apply max actions
    if opts.MaxActions > 0 && len(plan.Actions) > opts.MaxActions {
        plan.Remaining = len(plan.Actions) - opts.MaxActions
        plan.Actions = plan.Actions[:opts.MaxActions]
    }

    return plan, nil
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Reparación automática sin confirmación | --dry-run + --repair es más seguro; el usuario decide |
| Rollback de acciones filesystem | Difícil de implementar (mkdir no es reversible); mejor log + reporte |
| Repair como comando separado | Vivir dentro de `engram doctor --repair` es más natural que comando aparte |

## TDD plan

1. **Red:** RepairPlan identifica missing dirs → falla
2. **Green:** Implement generateRepairPlan → pasa
3. **Red:** Dry-run no ejecuta acciones → falla
4. **Green:** Implement DryRun check → pasa
5. **Red:** Stale sessions se cierran correctamente → falla
6. **Green:** Implement repairCloseStaleSessions → pasa
7. **Sabotaje:** No verificar precondición → acción se ejecuta aunque no necesaria → test idempotencia falla → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Missing dir repair crea estructura incorrecta | Solo crear el directorio exacto de la session; no árbol completo |
| Normalización de projects inconsistente con HU-08.2 | Reutilizar NormalizeProject() existente |
| Soft-delete orphans irreversible | No DELETE real; solo SET deleted_at; recuperable |
| Permisos insuficientes para crear directorios | Error capturado; otras acciones continúan |
