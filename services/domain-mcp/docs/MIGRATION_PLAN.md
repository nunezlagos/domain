# Plan de Migración Arquitectónica — domain-mcp

> Fecha: 2026-06-25  
> Rama base: `services`  
> Objetivo: eliminar acoplamiento estructural, violaciones SOLID, y funciones inmanejables sin romper nada.

---

## Diagnóstico — números concretos

| Problema               | Medida actual                                             | Objetivo              |
|------------------------|-----------------------------------------------------------|-----------------------|
| God Object REST        | `handler.API`: 55 campos concretos                        | ≤5 deps por handler   |
| God Object MCP         | `server.Deps`: 28 campos concretos                        | ≤5 deps por tool      |
| Función monolítica     | `runServer()`: ~879 líneas, 12+ responsabilidades         | ≤50 líneas por func   |
| OCP violado            | 19 `append(tools, registerXxx...)` manuales               | 0 modificaciones para agregar tool |
| DIP violado            | Handlers/tools dependen de `*ConcreteService`             | Todos dependen de interface |
| Boilerplate MCP        | 156 `GetArguments()` calls, 135 `Principal == nil` checks | Centralizado en middleware |

---

## Fase 0 — Corrección urgente (30 min)

**Problema**: `internal/service/agent/orchestration/templates_store.go:24` tiene `q()` sin `ctx` — ignora transacciones activas.

**Cambio**:
```go
// antes
func (s *TemplateStore) q() *agentdb.Queries { return agentdb.New(s.Pool) }

// después
func (s *TemplateStore) q(ctx context.Context) *agentdb.Queries {
    if tx := txctx.TxFromContext(ctx); tx != nil {
        return agentdb.New(tx)
    }
    return agentdb.New(s.Pool)
}
```

Agregar import `"nunezlagos/domain/internal/store/txctx"` y reemplazar todas las llamadas `s.q()` → `s.q(ctx)`.

**Archivos**: `internal/service/agent/orchestration/templates_store.go`  
**Riesgo**: Mínimo. Bug fix puro. Sin cambio de contrato externo.  
**Criterio**: `grep -n 'func.*q()' internal/service/agent/orchestration/templates_store.go` → sin resultados.

---

## Fase 1 — DIP: Romper God Objects (handler.API y server.Deps)

**Motivación**: Los handlers REST y tools MCP dependen del struct completo `handler.API` / `server.Deps`. Un cambio en cualquier servicio — aunque no lo use el handler — invalida y retestea TODO. Es la causa raíz del alto acoplamiento.

**Principio Go**: interfaces definidas en el consumidor, no en el productor. Pequeñas (1-3 métodos).

### 1.1 — Mapeo de handlers por acoplamiento real

Top 5 handlers más acoplados (medidos por referencias a `a.` campos):

| Handler                                    | Deps reales estimadas |
|--------------------------------------------|-----------------------|
| `internal/api/handler/ticket.go`           | ~8 servicios          |
| `internal/api/handler/rest_new.go`         | ~12 (router/dispatch) |
| `internal/api/handler/spec.go`             | ~6 servicios          |
| `internal/api/handler/flow.go`             | ~5 servicios          |
| `internal/api/handler/issue.go`            | ~5 servicios          |

### 1.2 — Estrategia de migración (handler.API)

**No se elimina `handler.API`** — es el punto de wiring. Lo que cambia es que cada handler recibe una interface pequeña en lugar de recibir `*API` directamente.

**Patrón a aplicar (ejemplo: ticket handlers)**:

```go
// internal/api/handler/ticket.go

// interfaces definidas en el consumidor
type ticketCreator interface {
    Create(ctx context.Context, in ticketsvc.CreateInput) (*ticketsvc.Ticket, error)
}
type ticketReader interface {
    GetByID(ctx context.Context, id uuid.UUID) (*ticketsvc.Ticket, error)
    List(ctx context.Context, orgID uuid.UUID, filter ticketsvc.ListFilter) ([]ticketsvc.Ticket, error)
}

type ticketHandlers struct {
    svc    ticketCreator // o composición ticketCreator + ticketReader
    audit  audit.Recorder
}

func newTicketHandlers(a *API) *ticketHandlers {
    return &ticketHandlers{svc: a.TicketService, audit: a.Audit}
}
```

El método `Router()` de `api.go` construye los sub-handlers con `newXxxHandlers(a)` y monta las rutas.

### 1.3 — Orden de ejecución Fase 1

Empezar por los más acoplados para mayor impacto inmediato:

1. `ticket.go` — handlers de tickets (REST)
2. `spec.go` — handlers de especificaciones
3. `flow.go` — handlers de flows
4. `issue.go` — handlers de issues/HUs
5. `rest_new.go` — router general (último, porque agrega rutas de los anteriores)

Para cada handler:
1. Identificar qué campos de `*API` usa realmente (grep `a\.XxxService`)
2. Definir interface(s) mínimas al tope del archivo
3. Crear struct `xxxHandlers` con solo esas deps
4. Agregar constructor `newXxxHandlers(a *API) *xxxHandlers`
5. Convertir métodos de handler de `func (a *API) handleXxx` → `func (h *xxxHandlers) handleXxx`
6. En `Router()`: usar `newXxxHandlers(a)` para construir y montar

### 1.4 — Estrategia de migración (server.Deps)

Mismo patrón para tools MCP. Cada archivo de tools define sus interfaces:

```go
// internal/mcp/server/ticket_tools.go

type ticketSvc interface {
    Create(ctx context.Context, in ticketsvc.CreateInput) (*ticketsvc.Ticket, error)
    GetByID(ctx context.Context, id uuid.UUID) (*ticketsvc.Ticket, error)
    // ...solo los métodos que este archivo usa
}
```

`server.Deps` se mantiene como struct de wiring, pero las funciones `registerXxx(wrap, deps)` reciben la interface, no `Deps`.

**Archivos a modificar**:
- `internal/api/handler/api.go` — solo agregar constructores, no eliminar campos aún
- `internal/api/handler/ticket.go`, `spec.go`, `flow.go`, `issue.go`, `rest_new.go`
- `internal/mcp/server/ticket_tools.go`, `issue_tools.go`, `policy_tools.go`, `flow_tools.go`, `skill_tools.go` (las 5 más grandes)

**Riesgo**: Medio. Cambio mecánico — renaming de receptores. Los tests existentes compilan igual porque `*API` sigue existiendo.  
**Criterio**: `grep -rn '\*API\b' internal/api/handler/*.go` → solo en `api.go` y `router()`. Cero en handlers individuales.

---

## Fase 2 — SRP: Split de runServer() (879 líneas → funciones ≤50 líneas)

**Motivación**: `runServer()` en `cmd/domain/main.go` hace config+logging+metrics+3 DB pools+audit+40 constructores de services+seeds+cipher+LLM factory+circuit breaker+runners+scheduler+router+8 goroutines de background. Cualquier cambio en cualquier servicio toca esta función.

### 2.1 — Nueva estructura de archivos

```
cmd/domain/
├── main.go                  # solo main() + runServer() como orquestador ≤50 líneas
├── server/
│   ├── config.go            # parsing de Config (ya existe como cfg, sin cambios)
│   ├── pools.go             # buildPools() — crea los 3 pgxpool con sus configs
│   ├── services.go          # buildServices() — 40+ constructores de services
│   ├── runners.go           # buildRunners() — agentRunner, flowRunner, scheduler, dispatcher
│   ├── wiring.go            # wireServices() — asignaciones cruzadas (obsService.Events = ..., etc.)
│   ├── router.go            # buildRouter() — monta HTTP handler.API + routes
│   ├── background.go        # startBackground() — lanza las 8+ goroutines
│   └── mcp.go               # buildMCPServer() — construye server.Deps + mcpserver
```

### 2.2 — runServer() después del split

```go
func runServer(ctx context.Context, cfg *Config) error {
    logger := buildLogger(cfg)
    pools, err := buildPools(ctx, cfg)
    if err != nil { return err }
    defer pools.Close()

    services, err := buildServices(ctx, cfg, pools, logger)
    if err != nil { return err }

    runners := buildRunners(ctx, cfg, services)
    wireServices(services, runners)

    httpHandler := buildRouter(cfg, services, runners)
    mcpServer := buildMCPServer(cfg, services, runners)

    return startBackground(ctx, cfg, runners, httpHandler, mcpServer)
}
```

### 2.3 — Función buildServices() — la más larga de las nuevas

Con ~40 constructores, `buildServices()` superará 50 líneas. Es la excepción documentada en AGENTS.md (wiring/DI en main). Se documenta con un comentario de una línea al inicio de la función.

### 2.4 — Orden de ejecución Fase 2

1. Crear `cmd/domain/server/pools.go` — extraer `pgxpoolNew` y los 3 pools (App, Auth, Admin)
2. Crear `cmd/domain/server/services.go` — extraer los 40+ constructores
3. Crear `cmd/domain/server/runners.go` — agentRunner, flowRunner, scheduler, dispatcher, leaderElection
4. Crear `cmd/domain/server/wiring.go` — asignaciones cruzadas post-construcción
5. Crear `cmd/domain/server/router.go` — `buildRouter()` que construye `handler.API` y monta rutas
6. Crear `cmd/domain/server/background.go` — `startBackground()` con las goroutines y signal handling
7. Reducir `runServer()` en `main.go` al esqueleto de 6-8 líneas

**Archivos a modificar**:
- `cmd/domain/main.go` — reducir de 879 líneas a ≤100 (solo configuración inicial + llamada a funciones)
- `cmd/domain/server/*.go` — nuevos archivos

**Riesgo**: Medio-alto. Mover código entre archivos puede perder estado implícito (variables `ctx`, `cfg`, closures). Hacer cambios incrementales: mover 1 grupo por commit, verificar que compila.  
**Criterio**: `wc -l cmd/domain/main.go` → ≤120 líneas. Cada función en `server/*.go` ≤50 líneas (excepto `buildServices`).

---

## Fase 3 — OCP: Auto-registro de tools MCP

**Motivación**: Agregar una nueva tool MCP requiere modificar `server.go` en 3 lugares (importar, agregar a Deps, agregar al `append`). Viola OCP — está cerrado a extensión, abierto a modificación.

### 3.1 — Situación actual

```go
// internal/mcp/server/server.go — Tools()
tools = append(tools, registerTicketTools(wrap, deps)...)
tools = append(tools, registerIssueTools(wrap, deps)...)
// ... 17 líneas más de append manual
```

### 3.2 — Patrón de auto-registro

Cada grupo de tools expone una función pública `Register`:

```go
// internal/mcp/server/ticket_tools.go
func RegisterTicketTools(wrap *ResilientWrapper, deps TicketDeps) []mcpgo.ServerTool {
    // ... mismo código que registerTicketTools, pero deps es interface no Deps
}
```

Una tabla en `server.go`:

```go
// internal/mcp/server/server.go
type toolRegistrar func(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool

var toolRegistrars = []toolRegistrar{
    func(w *ResilientWrapper, d Deps) []mcpgo.ServerTool { return RegisterTicketTools(w, d) },
    func(w *ResilientWrapper, d Deps) []mcpgo.ServerTool { return RegisterIssueTools(w, d) },
    // ...
}

func Tools(deps Deps) []mcpgo.ServerTool {
    wrap := NewResilientWrapper(defaultBudget)
    var tools []mcpgo.ServerTool
    for _, r := range toolRegistrars { tools = append(tools, r(wrap, deps)...) }
    return tools
}
```

Agregar una tool nueva = agregar su archivo + un entry en `toolRegistrars`. Cero modificación al cuerpo de `Tools()`.

### 3.3 — Orden de ejecución Fase 3

1. Renombrar los 19 `registerXxx` a `RegisterXxx` (exportados)
2. Crear interfaces mínimas `XxxDeps` por grupo (de Fase 1)
3. Actualizar firmas de `RegisterXxx` para recibir su interface
4. Construir `toolRegistrars` en `server.go`
5. Simplificar `Tools()` al loop

**Archivos a modificar**:
- `internal/mcp/server/server.go` — reemplazar 19 appends por tabla + loop
- `internal/mcp/server/*_tools.go` — renombrar funciones, actualizar firmas

**Riesgo**: Bajo. Cambio mecánico. Los tests de MCP siguen funcionando si `Tools()` retorna las mismas tools.  
**Criterio**: `grep -c 'append(tools,' internal/mcp/server/server.go` → 1 (solo el del loop).

---

## Orden de ejecución entre fases

```
Fase 0 (30 min)   →   Fase 1 (REST handlers, 3-4h)   →   Fase 1 (MCP tools, 2-3h)
                                                              ↓
                       Fase 3 (OCP, 2h)              ←   Fase 2 (SRP split, 4-5h)
```

- **Fase 0 primero**: bug real, sin dependencias.
- **Fase 1 antes que Fase 2**: las interfaces definidas en Fase 1 son las que usa `buildServices()` de Fase 2 para inyectar correctamente.
- **Fase 3 después de Fase 1**: depende de que las interfaces `XxxDeps` ya existan.
- **Fase 2 independiente** de Fase 3: se puede hacer en paralelo con Fase 3.

---

## Resumen de commits por fase

| Fase | Commits esperados | Estrategia |
|------|-------------------|------------|
| 0    | 1                 | `fix(agent/orchestration): q() sin ctx → q(ctx context.Context)` |
| 1a   | 5 (1 por handler) | `refactor(api/handler): DIP en ticket/spec/flow/issue/rest_new` |
| 1b   | 3 (por grupo)     | `refactor(mcp/server): interfaces por grupo de tools` |
| 2    | 7 (1 por archivo) | `refactor(cmd/domain): extraer pools/services/runners/wiring/router/background` |
| 3    | 2                 | `refactor(mcp/server): auto-registro de tools via toolRegistrars` |

**Total**: ~18 commits atómicos, cada uno compilable y con tests verdes.

---

## Qué NO forma parte de este plan

- Eliminar campos de `handler.API` o `server.Deps` — se mantienen como punto de wiring, solo se dejan de usar directamente en handlers.
- Reescribir tests existentes — los tests de integración siguen funcionando sin cambios.
- Cambiar la capa de DB (sqlc ya está migrado).
- Modificar el comportamiento de negocio — refactoring puro, cero cambios funcionales.
