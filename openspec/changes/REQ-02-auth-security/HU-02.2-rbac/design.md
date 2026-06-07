# Design: HU-02.2-rbac

## Decisión arquitectónica

**Matriz de permisos:** Map estático en `internal/auth/rbac/permissions.go`
**Roles:** Constantes tipadas con string
**Middleware:** Closure que recibe `entity, action`, verifica contra `RolePermissions[role]`
**Scoping:** Store methods que reciben `organizationID` y lo incluyen en WHERE. Handler extrae `organizationID` del contexto (inyectado por AuthMiddleware).

## Alternativas descartadas

- **Casbin:** Framework externo pesado para 3 roles fijos. No necesitamos policy engine dinámico.
- **Permisos en base de datos:** Overkill para roles fijos. Matriz hardcodeada es más simple y visible.
- **Middleware por ruta individual:** Demasiado boilerplate. Closure parameterizado es más limpio.

## Diagrama

```
Request → AuthMiddleware (HU-02.1) → [context: user_id, org_id, role]
  │
  └─→ RequirePermission("project", "delete")
        │
        └─→ Get role from context
        └─→ Lookup RolePermissions[role]
        └─→ Check "project:delete" ∈ permissions
              ├─→ YES → next handler
              └─→ NO  → 403 Forbidden

Handler → Store.Method(orgID, ...)
              │
              └─→ WHERE organization_id = orgID
              └─→ Si no pertenece → return nil (no error, como "not found")
```

## Matriz de permisos

```go
type Role string

const (
    RoleAdmin     Role = "admin"
    RoleDeveloper Role = "developer"
    RoleViewer    Role = "viewer"
)

type Permission string // format: "entity:action"

var RolePermissions = map[Role]map[Permission]bool{
    RoleAdmin: {
        "project:create": true, "project:read": true, "project:update": true, "project:delete": true,
        "flow:create": true,    "flow:read": true,    "flow:update": true,    "flow:delete": true,    "flow:execute": true,
        "agent:create": true,   "agent:read": true,   "agent:update": true,   "agent:delete": true,   "agent:execute": true,
        "skill:create": true,   "skill:read": true,   "skill:update": true,   "skill:delete": true,   "skill:execute": true,
        "knowledge_doc:create": true, "knowledge_doc:read": true, "knowledge_doc:update": true, "knowledge_doc:delete": true,
        "cron:create": true,    "cron:read": true,    "cron:update": true,    "cron:delete": true,
        "webhook:create": true, "webhook:read": true, "webhook:update": true, "webhook:delete": true,
        "api_key:create": true, "api_key:read": true, "api_key:update": true, "api_key:delete": true,
        "secret:create": true,  "secret:read": true,  "secret:update": true,  "secret:delete": true,
        "organization:read": true, "organization:update": true,
        "user:create": true,    "user:read": true,    "user:update": true,    "user:delete": true,
    },
    RoleDeveloper: {
        "project:read": true,
        "flow:create": true,    "flow:read": true,    "flow:update": true,    "flow:execute": true,
        "agent:create": true,   "agent:read": true,   "agent:update": true,   "agent:execute": true,
        "skill:create": true,   "skill:read": true,   "skill:update": true,   "skill:execute": true,
        "knowledge_doc:create": true, "knowledge_doc:read": true, "knowledge_doc:update": true,
        "cron:read": true,
        "webhook:read": true,
        "api_key:read": true,
        "secret:read": true,
    },
    RoleViewer: {
        "project:read": true,
        "flow:read": true,
        "agent:read": true,
        "skill:read": true,
        "knowledge_doc:read": true,
        "cron:read": true,
        "webhook:read": true,
        "api_key:read": true,
        "organization:read": true,
    },
}
```

## TDD plan

1. Test matriz admin tiene todos los permisos
2. Test matriz developer no tiene delete en ninguna entidad
3. Test matriz viewer solo tiene read permissions
4. Test RequirePermission pasa para permiso válido
5. Test RequirePermission rechaza para permiso inválido (403)
6. Test cross-org devuelve 404 (scoping)
7. Test default role es viewer
8. Test middleware integration con cada endpoint protegido

## Riesgos y mitigación

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|-------------|---------|------------|
| Cross-org data leak | Baja | Crítico | Toda query incluye WHERE organization_id; tests multi-org |
| Olvidar proteger un endpoint | Media | Alto | Safety check en code review; test que lista endpoints y verifica middleware |
| Developer puede leer secrets | Media | Alto | Developer role NO incluye "secret:read" en la matriz propuesta |
