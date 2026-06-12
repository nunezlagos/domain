# Design: HU-28.1-repository-interfaces

## Arquitectura

```
┌─────────────────────────────────────────────────────────────┐
│  cmd/domain/main.go                  (wiring)              │
│    repo := &pgFlowRepository{Pool: pools.App}              │
│    svc := flow.NewService(pools.App, recorder, repo)       │
└──────────────────────┬──────────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────────┐
│  service/flow/                                             │
│    Service {                                                │
│      Pool  *pgxpool.Pool  // legacy, deprecated            │
│      Audit audit.Recorder                                  │
│      repo  FlowRepository  // nueva dependencia            │
│    }                                                       │
│    func NewService(pool, audit, repo) *Service             │
│    func (s *Service) Create(ctx, f) error {                │
│        // validación de negocio                            │
│        id, err := s.repo.InsertFlow(ctx, f)                │
│        // audit + return                                   │
│    }                                                       │
└──────────────────────┬──────────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────────┐
│  service/flow/pg_repository.go                              │
│    pgFlowRepository { pool *pgxpool.Pool }                 │
│    func (r *pgFlowRepository) InsertFlow(ctx, f) error {   │
│        // SQL inline exactamente igual que hoy             │
│        err := r.pool.QueryRow(ctx, sql, args...).Scan(...) │
│    }                                                       │
└─────────────────────────────────────────────────────────────┘
```

## Interfaces por service

### `service/flow/repository.go`

```go
type FlowRepository interface {
    InsertFlow(ctx context.Context, f *Flow) (uuid.UUID, error)
    GetFlow(ctx context.Context, id uuid.UUID) (*Flow, error)
    UpdateFlow(ctx context.Context, f *Flow) error
    ListFlows(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*Flow, error)
    DeleteFlow(ctx context.Context, id uuid.UUID) error
}
```

### `service/agent/repository.go`

```go
type AgentRepository interface {
    InsertAgent(ctx context.Context, a *Agent) (uuid.UUID, error)
    GetAgent(ctx context.Context, id uuid.UUID) (*Agent, error)
    UpdateAgent(ctx context.Context, a *Agent) error
    ListAgents(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*Agent, error)
    DeleteAgent(ctx context.Context, id uuid.UUID) error
}
```

### `service/observation/repository.go`

```go
type ObservationRepository interface {
    InsertObservation(ctx context.Context, o *Observation) error
    GetObservation(ctx context.Context, id uuid.UUID) (*Observation, error)
    ListObservations(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]*Observation, error)
    UpdateObservation(ctx context.Context, o *Observation) error
}
```

### `service/session/repository.go`

```go
type SessionRepository interface {
    InsertSession(ctx context.Context, s *Session) (uuid.UUID, error)
    GetSession(ctx context.Context, id uuid.UUID) (*Session, error)
    EndSession(ctx context.Context, id uuid.UUID) error
    ListSessions(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*Session, error)
}
```

### `service/project/repository.go`

```go
type ProjectRepository interface {
    InsertProject(ctx context.Context, p *Project) (uuid.UUID, error)
    GetProject(ctx context.Context, id uuid.UUID) (*Project, error)
    UpdateProject(ctx context.Context, p *Project) error
    ListProjects(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*Project, error)
}
```

## Estrategia de migración

Por cada service, en orden:

1. Definir interfaz en `repository.go`
2. Implementar `pg_repository.go` con pool legacy (mover SQL inline tal cual)
3. Agregar constructor `NewService(pool, audit, repo)`
4. Migrar métodos UNO POR UNO de `s.Pool.QueryRow` → `s.repo.Method`
5. Cada commit migra 1 método + test unitario con mock

Orden sugerido: observation (más simple) → session → project → agent → flow (más complejo).
