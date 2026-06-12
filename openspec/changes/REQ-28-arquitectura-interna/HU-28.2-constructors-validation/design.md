# Design: HU-28.2-constructors-validation

## Patrón

```go
// service/flow/service.go

// Deprecated: Pool se mantiene público para backward-compat.
// Usar NewService para nuevo código.
type Service struct {
    Pool  *pgxpool.Pool
    Audit audit.Recorder
    repo  FlowRepository
}

func NewService(pool *pgxpool.Pool, audit audit.Recorder, repo FlowRepository) (*Service, error) {
    switch {
    case pool == nil:
        return nil, errors.New("flow: nil pool")
    case audit == nil:
        return nil, errors.New("flow: nil audit")
    case repo == nil:
        return nil, errors.New("flow: nil repository")
    }
    return &Service{
        Pool:  pool,
        Audit: audit,
        repo:  repo,
    }, nil
}
```

En los services sin repository interface (HU-28.1), el constructor solo valida pool + audit.
