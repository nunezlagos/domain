# HU-28.1-repository-interfaces

**Origen:** `REQ-28-arquitectura-interna`
**Prioridad tentativa:** alta
**Tipo:** refactor

## Historia de usuario

**Como** desarrollador de Domain
**Quiero** que los services dependan de interfaces de repositorio (`FlowRepository`, `AgentRepository`, etc.) en vez de `*pgxpool.Pool` directo
**Para** poder unit-testear lógica de negocio sin DB real, y poder cambiar el storage sin modificar 25+ services

## Contexto

Hoy todos los services tienen `Pool *pgxpool.Pool` como field público y escriben SQL inline. Esto es Active Record, no Repository. Consecuencias:
- Tests unitarios de lógica de negocio requieren DB real o testcontainers → feedback loop de segundos en vez de milisegundos
- Cambiar de Postgres a otra cosa implica tocar 25+ archivos
- No hay un único lugar donde auditar queries (ej: tracing, slow query logging)
- El field Pool es público y mutable — nada impide que otro package lo sobreescriba

Esta HU define interfaces para los 5 services más acoplados (flow, agent, observation, session, project), implementa la versión concreta PG (que basicamente wrappea el código existente), y migra los services a recibir la interfaz en vez del pool directo.

**NO** se mueve SQL a archivos separados ni se cambian las queries. Solo se introduce el contrato. El refactor de mover SQL a repositorios dedicados queda para HU futura si se necesita.

## Criterios de aceptación

### Escenario 1: Interfaces definidas en el package de servicio

```gherkin
Dado que existe el package `service/flow`
Cuando reviso las interfaces
Entonces existe `FlowRepository` en el mismo package (donde se usa, no donde se implementa)
Y la implementación concreta `pgFlowRepository` está en `service/flow/pg_repository.go`
Y `flow.Service` tiene campo `repo FlowRepository` en vez de `Pool *pgxpool.Pool`
```

### Escenario 2: Backward-compat — wiring existente no se rompe

```gherkin
Dado que `cmd/domain/main.go` construye `&flow.Service{Pool: pools.App, Audit: recorder}`
Cuando introduzco el constructor `flow.NewService(pool, audit, repo)`
Entonces el struct literal sigue compilando (Pool field se mantiene temporalmente como field público no-excluyente)
Y `NewService` asigna `Pool` internamente desde el pool parameter para legacy compat
```

### Escenario 3: Tests unitarios sin DB

```gherkin
Dado que `flow.Service.Create` usa `s.repo.InsertFlow(ctx, f)` en vez de `s.Pool.QueryRow`
Cuando escribo un test unitario
Entonces puedo pasar un `FlowRepository` mock
Y probar lógica de negocio sin levantar Postgres
```

### Escenario 4: Sabotaje — mock que siempre falla

```gherkin
Dado un mock de `FlowRepository` donde `InsertFlow` retorna `ErrNotFound`
Cuando `service.Create` lo llama
Entonces el error se propaga correctamente sin panic ni estado inconsistente
```

## Análisis breve

- **Qué pide:** Interfaces `FlowRepository`, `AgentRepository`, `ObservationRepository`, `SessionRepository`, `ProjectRepository` en sus respectivos packages `service/*/`. Implementación concreta PG que wrappea el pool actual. Constructores nuevos que aceptan la interfaz.
- **Módulos afectados:**
  - `internal/service/flow/` — interfaz + impl PG + refactor Service
  - `internal/service/agent/` — ídem
  - `internal/service/observation/` — ídem  
  - `internal/service/session/` — ídem
  - `internal/service/project/` — ídem
  - `cmd/domain/main.go` — wiring: pasar repo concreto al constructor
  - `cmd/domain-mcp/main.go` — ídem
- **Riesgos:**
  - El field `Pool` público coexiste temporalmente (Strangler Fig). Nadie debe usarlo nuevo código, pero el viejo sigue funcionando.
  - Tests existentes que usan `&Service{Pool: mockPool}` siguen compilando porque el field sigue ahí.
  - Las implementaciones PG concretas son wrappers del código existente — riesgo de regression bajo si se testea cada método.
- **Esfuerzo tentativo:** L (3-4 días, ~5 services × ~5 métodos c/u)
- **Dependencias:** Ninguna. Puede correr en paralelo con otras HUs.

## TDD plan

1. **Red:** Test unitario por service que usa mock repository y verifica lógica de negocio (falla porque no existe la interfaz).
2. **Green:** Definir interfaz + impl PG concreta + constructor nuevo.
3. **Refactor:** Migrar un método del service a la interfaz por commit, tests verdes en cada paso.
4. **Sabotaje:** Mock que retorna error — service lo propaga correctamente.
