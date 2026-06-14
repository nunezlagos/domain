# Proposal: issue-13.1-http-crud-endpoints

## Intención

Implementar una API REST completa bajo `/api/v1/` que exponga operaciones CRUD para las 16 entidades del sistema: organizations, users, api_keys, projects, observations, sessions, prompts, knowledge_docs, skills, agents, agent_runs, flows, flow_runs, crons, webhooks, audit_log. Cada entidad sigue el mismo patrón de rutas, códigos de respuesta, validación y formato de errores.

## Scope

**Incluye:**
- Router REST con handler factory (evitar repetir handler por entidad)
- Handlers: POST /{entity}, GET /{entity}, GET /{entity}/{id}, PUT /{entity}/{id}, PATCH /{entity}/{id}, DELETE /{entity}/{id}
- Validación de request body con schemas JSON (por entidad)
- Serialización consistente: camelCase, timestamps ISO8601, UUIDs como string
- Formato de error uniforme: `{error: {code, message, details?}}`
- Response envelope para listas: `{data: [...], pagination: {...}}`
- Swagger/OpenAPI spec generada automáticamente
- Health check endpoint: GET /api/v1/health

**Excluye:**
- Autenticación (se cubre en issue-13.2)
- Paginación avanzada (se cubre en issue-13.3)
- Rate limiting (se cubre en issue-13.2)

## Enfoque técnico

**Handler Factory Pattern:**
```go
type CRUDHandlers[T Entity] struct {
    store   Store[T]
    schema  ValidationSchema
    expose  []string // qué endpoints exponer
}

func NewCRUD[T Entity](r *mux.Router, prefix string, h CRUDHandlers[T]) {
    r.HandleFunc(prefix, h.Create).Methods("POST")
    r.HandleFunc(prefix, h.List).Methods("GET")
    r.HandleFunc(prefix+"/{id}", h.Get).Methods("GET")
    r.HandleFunc(prefix+"/{id}", h.Update).Methods("PUT")
    r.HandleFunc(prefix+"/{id}", h.Patch).Methods("PATCH")
    r.HandleFunc(prefix+"/{id}", h.Delete).Methods("DELETE")
}
```

**Store interface genérica:**
```go
type Store[T Entity] interface {
    Create(ctx, entity) (T, error)
    GetByID(ctx, id) (T, error)
    List(ctx, filters) ([]T, error)
    Update(ctx, id, entity) (T, error)
    Delete(ctx, id) error
}
```

**Validación:**
- Schemas por entidad definidos como structs Go con tags `validate` (usando `go-playground/validator`)
- Validación centralizada en middleware antes de llegar al handler

**Routing:**
- `gorilla/mux` o `chi` para path params y middlewares
- Subrouter por entidad con prefijo `/api/v1/{entity}`

## Riesgos

| Riesgo | Mitigación |
|--------|------------|
| Handler genérico demasiado rígido para edge cases | Permitir hooks de pre/post procesamiento por entidad |
| 16 entidades = 96 endpoints, volumen de testing enorme | Tests parametrizados (table-driven) que iteran sobre lista de entidades |
| Put vs Patch confusión semántica | PUT = reemplazo total (requiere todos los campos), PATCH = merge parcial (solo campos enviados) |
| Performance con joins complejos en listados | Lazy loading de relaciones, solo incluir si query param `?include=...` |

## Testing

- Table-driven tests: un test por operación que itera sobre todas las entidades
- Golden files para response bodies de cada entidad
- E2E: levantar servidor de test, ejecutar ciclo CRUD completo contra cada entidad
- Sabotaje: enviar bodies inválidos, IDs inexistentes, métodos no permitidos
