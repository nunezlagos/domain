# Proposal: HU-02.2-rbac

## Intención

Implementar control de acceso basado en roles (RBAC) con tres niveles: admin, developer, viewer. Cada rol tiene una matriz de permisos sobre entidades del sistema. El middleware de autorización valida que el usuario autenticado tenga el permiso requerido para la acción solicitada, siempre dentro del scope de su organización.

## Scope

**Incluye:**
- Definición de roles: admin, developer, viewer
- Matriz de permisos: create, read, update, delete, execute
- Permisos por entidad: project, flow, agent, skill, knowledge_doc, cron, webhook, api_key, secret, organization, user
- Middleware `RequirePermission(entity, action)` que lee del context el rol del usuario
- Scoping por organización: toda query incluye `WHERE organization_id = ?`
- Helper `GetOrganizationID` en store que verifica pertenencia antes de operar
- Tests de integración con multi-org

**No incluye:**
- Roles personalizados (custom roles)
- Hierarchy de roles con herencia (admin hereda todos)
- Permission overrides a nivel de recurso individual
- UI para gestión de roles

## Enfoque técnico

1. Matriz de permisos hardcodeada como map estático
2. Cada permiso es un string: `entity:action` ej: `project:delete`
3. `RolePermissions[role]` devuelve set de permisos
4. Middleware recibe entity y action como parámetros, consulta `RolePermissions[user.Role]`
5. Scoping: store methods reciben `organizationID` y lo incluyen en WHERE
6. `internal/auth/rbac/rbac.go` con lógica pura (sin I/O)

## Riesgos

- **403 vs 404 en cross-org:** Usar 404 para no revelar existencia de recursos en otras orgs.
- **Rol no asignado al crear usuario:** Default `viewer` por defecto.
- **Permisos inconsistentes entre middleware y handler:** El middleware es la única puerta de entrada.
- **Olvidar scoping en store queries:** Las queries siempre deben incluir `organization_id`.

## Testing

- Test unitario de matriz de permisos
- Test middleware con cada rol y acción
- Test scoping multi-org (mismo resource ID en org A y org B)
- Test default role es viewer
- Test 403 vs 404 en cross-org
