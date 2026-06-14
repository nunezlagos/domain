# Tasks: issue-02.2-rbac

## Backend

- [x] Crear `internal/auth/rbac/role.go` con tipo Role y constantes
- [x] Crear `internal/auth/rbac/permissions.go` con matriz de permisos
- [x] Crear función `HasPermission(role Role, entity string, action string) bool`
- [x] Crear middleware `RequirePermission(entity, action string)` en `internal/api/middleware/rbac.go`
- [x] Integrar RBAC middleware en cada ruta protegida del router
- [x] Agregar scoping por organization_id en todos los store methods
- [x] Asegurar que 404 vs 403 en cross-org se maneja correctamente
- [x] Agregar `role` al contexto de autenticación (desde user record)
- [x] Default role `viewer` al crear usuario

## Tests

- [x] Test unitario: matriz admin tiene todos los permisos
- [x] Test unitario: developer no tiene delete
- [x] Test unitario: viewer solo read
- [x] Test unitario: HasPermission true/false
- [x] Test unitario: HasPermission con role inválido
- [x] Test middleware: 200 para permiso válido
- [x] Test middleware: 403 para permiso inválido
- [x] Test integración: cross-org devuelve 404
- [x] Test integración: default role viewer
- [x] Sabotaje: agregar "secret:read" a developer → confirmar que test cae → restaurar
- [x] Sabotaje: no incluir organization_id en query → confirmar que test multi-org cae → restaurar

## Cierre

- [x] Verificación manual con diferentes roles
- [x] Suite verde
