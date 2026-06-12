# Tasks: issue-02.1-api-key-auth

## Backend

- [x] Crear `internal/auth/keygen.go` con generación de key usando `crypto/rand`
- [x] Crear `internal/auth/store.go` con interface APIKeyStore e implementación Postgres
- [x] Implementar `Create`: generar key, hashear bcrypt, guardar, retornar key original
- [x] Implementar `Authenticate`: lookup por key_prefix, bcrypt verify, check revoked/expired
- [x] Implementar `ListByOrganization`: listar sin key_hash ni key original
- [x] Implementar `Delete`: borrado físico
- [x] Implementar `Rotate`: nueva key + revocar anterior (transacción)
- [x] Implementar `Revoke`: set revoked_at = now()
- [x] Crear middleware HTTP `AuthMiddleware` en `internal/api/middleware/auth.go`
- [x] Agregar dependencia `golang.org/x/crypto/bcrypt`
- [x] Registrar endpoints CRUD en router
- [x] Sanitizar logs para no exponer keys

## Tests

- [x] Test unitario: keygen formato y largo
- [x] Test unitario: bcrypt hash/verify
- [x] Test unitario: Authenticate OK
- [x] Test unitario: Authenticate con key inválida
- [x] Test unitario: Authenticate con key expirada
- [x] Test unitario: Authenticate con key revocada
- [x] Test unitario: Create devuelve key original (una vez)
- [x] Test unitario: Rotate atómico
- [x] Test unitario: List sin datos sensibles
- [x] Test middleware: 401 en cada caso de error
- [x] Sabotaje: cambiar bcrypt.Compare para aceptar cualquier key → confirmar que test cae → restaurar
- [x] Sabotaje: no checkear revoked_at → confirmar que test cae → restaurar

## Cierre

- [x] Verificación manual con curl: crear key, autenticarse, listar, rotar
- [x] Suite verde
