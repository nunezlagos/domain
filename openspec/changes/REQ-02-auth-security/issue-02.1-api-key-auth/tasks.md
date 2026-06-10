# Tasks: issue-02.1-api-key-auth

## Backend

- [ ] Crear `internal/auth/keygen.go` con generación de key usando `crypto/rand`
- [ ] Crear `internal/auth/store.go` con interface APIKeyStore e implementación Postgres
- [ ] Implementar `Create`: generar key, hashear bcrypt, guardar, retornar key original
- [ ] Implementar `Authenticate`: lookup por key_prefix, bcrypt verify, check revoked/expired
- [ ] Implementar `ListByOrganization`: listar sin key_hash ni key original
- [ ] Implementar `Delete`: borrado físico
- [ ] Implementar `Rotate`: nueva key + revocar anterior (transacción)
- [ ] Implementar `Revoke`: set revoked_at = now()
- [ ] Crear middleware HTTP `AuthMiddleware` en `internal/api/middleware/auth.go`
- [ ] Agregar dependencia `golang.org/x/crypto/bcrypt`
- [ ] Registrar endpoints CRUD en router
- [ ] Sanitizar logs para no exponer keys

## Tests

- [ ] Test unitario: keygen formato y largo
- [ ] Test unitario: bcrypt hash/verify
- [ ] Test unitario: Authenticate OK
- [ ] Test unitario: Authenticate con key inválida
- [ ] Test unitario: Authenticate con key expirada
- [ ] Test unitario: Authenticate con key revocada
- [ ] Test unitario: Create devuelve key original (una vez)
- [ ] Test unitario: Rotate atómico
- [ ] Test unitario: List sin datos sensibles
- [ ] Test middleware: 401 en cada caso de error
- [ ] Sabotaje: cambiar bcrypt.Compare para aceptar cualquier key → confirmar que test cae → restaurar
- [ ] Sabotaje: no checkear revoked_at → confirmar que test cae → restaurar

## Cierre

- [ ] Verificación manual con curl: crear key, autenticarse, listar, rotar
- [ ] Suite verde
