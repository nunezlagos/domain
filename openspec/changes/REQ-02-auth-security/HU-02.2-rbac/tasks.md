# Tasks: HU-02.2-rbac

## Backend

- [ ] Crear `internal/auth/rbac/role.go` con tipo Role y constantes
- [ ] Crear `internal/auth/rbac/permissions.go` con matriz de permisos
- [ ] Crear función `HasPermission(role Role, entity string, action string) bool`
- [ ] Crear middleware `RequirePermission(entity, action string)` en `internal/api/middleware/rbac.go`
- [ ] Integrar RBAC middleware en cada ruta protegida del router
- [ ] Agregar scoping por organization_id en todos los store methods
- [ ] Asegurar que 404 vs 403 en cross-org se maneja correctamente
- [ ] Agregar `role` al contexto de autenticación (desde user record)
- [ ] Default role `viewer` al crear usuario

## Tests

- [ ] Test unitario: matriz admin tiene todos los permisos
- [ ] Test unitario: developer no tiene delete
- [ ] Test unitario: viewer solo read
- [ ] Test unitario: HasPermission true/false
- [ ] Test unitario: HasPermission con role inválido
- [ ] Test middleware: 200 para permiso válido
- [ ] Test middleware: 403 para permiso inválido
- [ ] Test integración: cross-org devuelve 404
- [ ] Test integración: default role viewer
- [ ] Sabotaje: agregar "secret:read" a developer → confirmar que test cae → restaurar
- [ ] Sabotaje: no incluir organization_id en query → confirmar que test multi-org cae → restaurar

## Cierre

- [ ] Verificación manual con diferentes roles
- [ ] Suite verde
