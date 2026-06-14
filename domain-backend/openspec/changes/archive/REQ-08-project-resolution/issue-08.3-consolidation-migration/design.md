# Design: issue-08.3-consolidation-migration

## Decisión arquitectónica

### Core: ConsolidateProjects

```go
type ConsolidateOpts struct {
    DryRun bool
}

type ConsolidateResult struct {
    Success             bool   `json:"success"`
    MigratedSessions    int64  `json:"migrated_sessions"`
    MigratedObservations int64 `json:"migrated_observations"`
    MigratedPrompts     int64  `json:"migrated_prompts,omitempty"`
    DryRun              bool   `json:"dry_run"`
    Error               string `json:"error,omitempty"`
}

func ConsolidateProjects(ctx context.Context, db *sql.DB, from, to string, opts ConsolidateOpts) (*ConsolidateResult, error) {
    if from == to {
        return nil, fmt.Errorf("source and destination must be different")
    }
    // Verificar que origen existe
    // Verificar que destino existe

    if opts.DryRun {
        return dryRunConsolidate(ctx, db, from, to)
    }

    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback() // safe; no-op si ya commiteó

    result := &ConsolidateResult{DryRun: false}

    // Migrar sesiones
    res, err := tx.ExecContext(ctx,
        "UPDATE sessions SET project = ? WHERE project = ?", to, from)
    // ... count rows affected

    // Migrar observaciones
    res, err = tx.ExecContext(ctx,
        "UPDATE observations SET project = ? WHERE project = ?", to, from)

    // Migrar prompts
    res, err = tx.ExecContext(ctx,
        "UPDATE user_prompts SET project = ? WHERE project = ?", to, from)

    if err := tx.Commit(); err != nil {
        return nil, err
    }

    result.Success = true
    return result, nil
}
```

### Dry-run mode

```go
func dryRunConsolidate(ctx context.Context, db *sql.DB, from, to string) (*ConsolidateResult, error) {
    result := &ConsolidateResult{DryRun: true}
    db.QueryRowContext(ctx,
        "SELECT COUNT(*) FROM sessions WHERE project = ?", from).Scan(&result.MigratedSessions)
    db.QueryRowContext(ctx,
        "SELECT COUNT(*) FROM observations WHERE project = ?", from).Scan(&result.MigratedObservations)
    db.QueryRowContext(ctx,
        "SELECT COUNT(*) FROM user_prompts WHERE project = ?", from).Scan(&result.MigratedPrompts)
    return result, nil
}
```

### HTTP endpoint

```
POST /api/projects/migrate
Content-Type: application/json

{
    "from": "myapp",
    "to": "my-app",
    "dry_run": false
}

Response 200:
{
    "success": true,
    "migrated_sessions": 12,
    "migrated_observations": 145,
    "migrated_prompts": 8,
    "dry_run": false
}
```

### CLI interactive

```
$ engram projects consolidate --interactive

? Found 3 candidate projects for consolidation:
  1. "my-app" (45 obs, 5 sessions) ← suggested destination
  2. "myapp" (12 obs, 2 sessions) ← suggested source (Levenshtein distance: 1)
  3. "my_app" (3 obs, 1 session) ← suggested source (underscore variant)
? Select source project: myapp
? Select destination project: my-app
? Proceed with consolidation? (Yes/No/Dry-run)
```

### mem_merge_projects tool function

La tool function permite a agentes llamar consolidación directamente:

```go
// Tool definition for agent protocol
{
    Name: "mem_merge_projects",
    Description: "Merge observations from source project into destination project",
    Parameters: {
        "from": "string (source project name)",
        "to": "string (destination project name)",
    },
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Migración por batches (paginada) | No necesario para volúmenes típicos (< 1M rows); transacción simple es más simple y veloz |
| DELETE de origen después de migrar | Riesgo de pérdida de datos; preferimos mantener los registros con project actualizado |
| Migración vía MCP tool exclusivamente | También debe haber CLI y HTTP para usuarios sin agente |

## TDD plan

1. **Red:** ConsolidateProjects migra sesiones y observations → falla
2. **Green:** Implement UPDATE queries + transacción → pasa
3. **Red:** Dry-run no modifica DB → falla
4. **Green:** Implement SELECT COUNT en lugar de UPDATE → pasa
5. **Red:** POST /projects/migrate retorna 200 con resultado → falla
6. **Green:** Implement handler → pasa
7. **Sabotaje:** Romper transacción (no Commit) → test de persistencia falla → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Transacción larga lockea DB | Batch size implícito (una sola transacción); para millones de rows, considerar chunks de 10k |
| Origen = destino | Validación upfront; error "source and destination must be different" |
| Origen no existe | SELECT COUNT(*) before; error 404 si es 0 |
| Race condition: sesión se crea durante migración | Transacción serializable o lock advisory; poco probable en uso single-user |
