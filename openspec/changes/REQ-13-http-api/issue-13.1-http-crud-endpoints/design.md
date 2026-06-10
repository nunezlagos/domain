# Design: issue-13.1-http-crud-endpoints

## Decisión arquitectónica

**Patrón elegido:** Generic Handler Factory + Entity Registry

```
┌─────────────────────────────────────────────────────────────┐
│                         API Router                          │
│  /api/v1/{entity} ──► EntityRegistry.Lookup(entity) ──►    │
│                      CRUDHandlers[T].Create/List/Get/etc    │
└─────────────────────────────────────────────────────────────┘
```

**Entity Registry:** Mapa centralizado que asocia nombre de entidad a su store, schema de validación, y opciones de exposición:

```go
type EntityInfo struct {
    Name      string
    Store     any  // Store[T] tipado
    Schema    *ValidationSchema
    Expose    EndpointSet // qué endpoints habilitar
    PreHooks  []HandlerHook
    PostHooks []HandlerHook
}

var EntityRegistry = map[string]EntityInfo{
    "observations": {
        Store: observationStore,
        Schema: observationSchema,
        Expose: {Create: true, List: true, Get: true, Update: true, Patch: true, Delete: true},
    },
    "audit_log": {
        Store: auditLogStore,
        Schema: auditLogSchema,
        Expose: {List: true, Get: true}, // solo lectura
    },
    // ... 14 más
}
```

**Response envelope genérico:**

```go
type APIResponse[T any] struct {
    Data       T              `json:"data"`
    Pagination *PaginationMeta `json:"pagination,omitempty"`
}

type APIError struct {
    Error struct {
        Code    string `json:"code"`
        Message string `json:"message"`
        Details any    `json:"details,omitempty"`
    } `json:"error"`
}
```

## Alternativas descartadas

1. **Handler manual por entidad:** Demasiado boilerplate repetitivo para 16 entidades. Mismo patrón una y otra vez → error prone.
2. **Code generation:** Podría generar handlers desde schemas, pero introduce paso de build adicional y complejidad de tooling. Factory pattern es más simple y mantenible.
3. **GraphQL:** Sobredimensionado para CRUD simple. REST es más estándar para integración externa. Podemos agregar GraphQL después si hace falta.

## Diagrama

```
POST   /api/v1/{entity}           → Create(entity)
GET    /api/v1/{entity}           → List(entity, query)
GET    /api/v1/{entity}/{id}      → Get(entity, id)
PUT    /api/v1/{entity}/{id}      → Update(entity, id, body)
PATCH  /api/v1/{entity}/{id}      → Patch(entity, id, body)
DELETE /api/v1/{entity}/{id}      → Delete(entity, id)

Middleware chain: RequestLogger → ValidateBody → Auth → Handler → ResponseWriter
```

## TDD plan

1. **Red:** Escribir test `TestCRUD_Create` que envía POST a `/api/v1/observations` y espera 201
2. **Green:** Implementar handler factory mínimo con solo Create para observations
3. **Refactor:** Generalizar a todas las entidades usando Registry
4. **Iterar:** Para cada operación (List, Get, Update, Patch, Delete)
5. **Sabotaje:** Cambiar handler para que devuelva 200 en create → test cae → restaurar

**Tests clave:**
- `TestCRUD_Create_AllEntities` — parametrizado, cada entidad tiene su fixture
- `TestCRUD_Get_NotFound` — 404 consistente
- `TestCRUD_Patch_PartialUpdate` — solo campos enviados cambian
- `TestCRUD_Validation_Errors` — 422 con detalles por campo
- `TestCRUD_Response_Consistency` — misma estructura sin importar entidad

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Type erasure en Go (no hay generics reales hasta 1.18) | Usar `any` con type assertions internas o code generation si es necesario |
| Put vs Patch semántica mal implementada | PUT: validar que todos los campos requeridos estén presentes; PATCH: validar solo los enviados |
| Entity Registry crece sin control | Tests de registro: cada entidad nueva debe tener al menos un test que verifique su URL |
| Algunas entidades necesitan endpoints especiales no-CRUD | Hook system en el registry: `ExtraHandlers func(r *mux.Router)` para rutas adicionales |
