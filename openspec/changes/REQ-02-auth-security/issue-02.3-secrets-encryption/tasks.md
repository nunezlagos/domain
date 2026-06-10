# Tasks: issue-02.3-secrets-encryption

## Backend

- [ ] Crear `internal/crypto/cipher.go` con AES-256-GCM encrypt/decrypt
- [ ] Implementar `NewAESGCMCipher(key string)` con key derivation
- [ ] Implementar formato nonce(12) || ciphertext || tag(16) con Base64
- [ ] Crear `internal/secrets/store.go` con SecretsStore interface e implementación Postgres
- [ ] Integrar encrypt antes de INSERT en store.Set
- [ ] Integrar decrypt después de SELECT en store.Get
- [ ] Agregar columna `encryption_key_id` a migración de secrets
- [ ] Implementar re-encrypt en `internal/secrets/reencrypt.go`
- [ ] Agregar comando CLI `domain secrets re-encrypt`
- [ ] Validar en startup que DOMAIN_ENCRYPTION_KEY tenga al menos 32 bytes
- [ ] Agregar endpoint CRUD para secrets (protegido con RBAC)

## Tests

- [ ] Test unitario: encrypt/decrypt roundtrip
- [ ] Test unitario: nonce uniqueness (1000 veces)
- [ ] Test unitario: decrypt con key diferente falla
- [ ] Test unitario: decrypt con ciphertext corrupto falla
- [ ] Test unitario: key derivation 16→32 bytes
- [ ] Test unitario: key derivation con 32 bytes = sin cambios
- [ ] Test unitario: store.Set encripta
- [ ] Test unitario: store.Get desencripta
- [ ] Test unitario: re-encrypt cambia encryption_key_id
- [ ] Sabotaje: cambiar nonce fijo → confirmar que uniqueness test cae → restaurar
- [ ] Sabotaje: skip decrypt → confirmar que store.Get devuelve basura → restaurar

## Cierre

- [ ] Verificación manual: crear secret, leerlo, verificar que está encriptado en DB
- [ ] Suite verde
