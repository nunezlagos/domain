# Proposal: HU-28.1-repository-interfaces

## Intención

Introducir el patrón Repository en los 5 services más acoplados sin cambiar su implementación interna. La interfaz se define en el package del service (donde se usa), la impl PG concreta wrappea el pool existente. Strangler Fig: el field Pool público se mantiene temporalmente, el nuevo código usa la interfaz.

## Scope

**Incluye:**
- Interface `FlowRepository` en `service/flow/` con métodos: `InsertFlow`, `GetFlow`, `UpdateFlow`, `ListFlows`, `DeleteFlow`
- Interface `AgentRepository` en `service/agent/`: `InsertAgent`, `GetAgent`, `UpdateAgent`, `ListAgents`, `DeleteAgent`
- Interface `ObservationRepository` en `service/observation/`: `InsertObservation`, `GetObservation`, `ListObservations`, `UpdateObservation`
- Interface `SessionRepository` en `service/session/`: `InsertSession`, `GetSession`, `EndSession`, `ListSessions`
- Interface `ProjectRepository` en `service/project/`: `InsertProject`, `GetProject`, `UpdateProject`, `ListProjects`
- Implementación concreta `pgFlowRepository` en `service/flow/pg_repository.go` (wrappea pool, SQL inline igual que hoy)
- Constructor `flow.NewService(pool, audit, repo FlowRepository)` que asigna `Pool` internamente
- Migración de `cmd/domain/main.go` a los nuevos constructores
- Tests unitarios con mocks para cada service

**No incluye:**
- Mover SQL a archivos separados de queries
- Extraer queries a package `db/` o similar
- Cambiar la implementación de las queries existentes
- Repository interfaces para otros services fuera de los 5 priorizados
- Refactor de los Store structs (DLQStore, SignalStore, etc.) dentro de service/flow/

## Enfoque técnico

```
// service/flow/repository.go
type FlowRepository interface {
    InsertFlow(ctx context.Context, f *Flow) error
    GetFlow(ctx context.Context, id uuid.UUID) (*Flow, error)
    UpdateFlow(ctx context.Context, f *Flow) error
    ListFlows(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*Flow, error)
    DeleteFlow(ctx context.Context, id uuid.UUID) error
}

// service/flow/pg_repository.go
type pgFlowRepository struct {
    pool *pgxpool.Pool
}

// service/flow/service.go
type Service struct {
    Pool  *pgxpool.Pool // legacy, se depreca
    Audit audit.Recorder
    repo  FlowRepository // nuevo
}

func NewService(pool *pgxpool.Pool, audit audit.Recorder, repo FlowRepository) *Service {
    return &Service{
        Pool:  pool,
        Audit: audit,
        repo:  repo,
    }
}
```

Migración por strangler: cada método de Service se actualiza de `s.Pool.QueryRow(...)` a `s.repo.GetFlow(...)` uno por uno, tests verdes en cada commit.

## Riesgos

| Riesgo | Mitigación |
|--------|-----------|
| Regression en queries existentes | Cada migración de método tiene su propio test (RED antes del cambio) |
| Field Pool público sigue siendo usado por código externo | Se mantiene hasta HU futura que elimine los últimos consumidores |
| Tests existentes usan struct literal con Pool | Pool field sigue ahí, compilan sin cambios |

## Testing

- **Unit:** New constructor → service creado con mock repository
- **Unit (cada método):** mock repository → lógica de negocio testable
- **Integration:** pgFlowRepository con testcontainer verifica queries reales
- **Sabotaje:** mock que retorna error → error propagado sin crash

## Rollback plan

Revertir commits por método individual. Cada commit es atómico (1 método migrado + su test).
