# Design: issue-10.3-conflict-cli-api

## Decisión arquitectónica

### CLI commands

```
engram conflicts list [--status] [--relation] [--limit] [--offset] [--json]
  → Query memory_relations con filtros, output table

engram conflicts show <id>
  → SELECT con JOIN a observations para contenido

engram conflicts stats
  → SELECT COUNT GROUP BY judgment_status, relation

engram conflicts scan [--dry-run] [--apply] [--max-insert] [--since] [--threshold]
  → Llama FindCandidates (issue-10.1)

engram conflicts deferred [list|show|replay] [id]
  → CRUD sobre sync_apply_deferred
```

### HTTP endpoints

```
GET    /conflicts?status=pending&relation=candidate&limit=20&offset=0
GET    /conflicts/:id
POST   /conflicts/:id/judge              → ejecuta JudgeBySemantic
POST   /conflicts/scan                   → ejecuta FindCandidates
GET    /conflicts/deferred               → lista deferred queue
GET    /conflicts/deferred/:id           → detalle deferred
POST   /conflicts/deferred/:id/replay    → reintenta deferred
```

### Shared service layer

```go
// internal/conflict/service.go

type ConflictService struct {
    db *sql.DB
}

func (s *ConflictService) ListConflicts(ctx context.Context, filter ConflictFilter) ([]MemoryRelation, int, error) {
    // Shared query logic for both CLI and HTTP
}

func (s *ConflictService) GetConflict(ctx context.Context, id int64) (*MemoryRelationDetail, error) {
    // JOIN con observations
}

func (s *ConflictService) GetStats(ctx context.Context) (*ConflictStats, error) {
    // GROUP BY queries
}

func (s *ConflictService) ListDeferred(ctx context.Context, filter DeferredFilter) ([]SyncApplyDeferred, error) {
    // Query sync_apply_deferred
}

func (s *ConflictService) ReplayDeferred(ctx context.Context, id string) error {
    // Re-intentar aplicar un deferred entry
}
```

### Deferred replay

```go
func (s *ConflictService) ReplayDeferred(ctx context.Context, id string) error {
    var entry SyncApplyDeferred
    err := s.db.QueryRowContext(ctx,
        "SELECT sync_id, entity, payload FROM sync_apply_deferred WHERE sync_id = ?", id,
    ).Scan(&entry.SyncID, &entry.Entity, &entry.Payload)
    if err != nil {
        return fmt.Errorf("deferred entry not found: %w", err)
    }

    err = applyDeferredPayload(ctx, s.db, entry)
    if err != nil {
        // Increment retry_count, update last_error
        s.db.ExecContext(ctx,
            `UPDATE sync_apply_deferred SET
             retry_count = retry_count + 1,
             last_error = ?,
             last_attempted_at = datetime('now')
             WHERE sync_id = ?`,
            err.Error(), id)
        return err
    }

    // Success: remove from deferred queue
    s.db.ExecContext(ctx, "DELETE FROM sync_apply_deferred WHERE sync_id = ?", id)
    return nil
}
```

### ConflictFilter

```go
type ConflictFilter struct {
    Status   string `json:"status,omitempty"`    // pending, judged, error
    Relation string `json:"relation,omitempty"`  // candidate, supersedes, conflicts_with, duplicate, unrelated
    Limit    int    `json:"limit,omitempty"`      // default 20
    Offset   int    `json:"offset,omitempty"`
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| CLI y HTTP con lógica separada | Shared service layer evita duplicación; ambos son thin wrappers |
| Offset pagination | Suficiente para volumen de conflictos (< 10k); keyset no necesario |

## TDD plan

1. **Red:** ListConflicts retorna filtered results → falla
2. **Green:** Implement ConflictService.ListConflicts → pasa
3. **Red:** POST /conflicts/scan ejecuta FindCandidates → falla
4. **Green:** Implement scan handler → pasa
5. **Red:** Deferred replay update retry_count → falla
6. **Green:** Implement replay logic → pasa

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Offset pagination ineficiente con muchos datos | Acceptable para < 10k conflictos; migrar a keyset si necesario |
| Replay exitoso no limpia deferred | DELETE después de éxito; log warning si falla DELETE |
