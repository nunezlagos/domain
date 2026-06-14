# Proposal: issue-02.3-secrets-encryption

## Intención

Proteger secrets almacenados (API keys de LLM, webhook secrets, tokens OAuth) mediante encriptación AES-256-GCM. La encriptación/desencriptación debe ser transparente para el resto del sistema: el store encripta al escribir y desencripta al leer.

## Scope

**Incluye:**
- Package `internal/crypto/cipher.go` con `Encrypt(plaintext []byte) (ciphertext []byte, err error)` y `Decrypt(ciphertext []byte) (plaintext []byte, err error)`
- AES-256-GCM con nonce de 12 bytes generado con `crypto/rand`
- Clave derivada directamente de `DOMAIN_ENCRYPTION_KEY` (32 bytes = SHA-256 si es más larga/corta)
- Almacenamiento en columna `encrypted_value` como Base64
- Columna `encryption_algorithm` = `aes-256-gcm`
- Columna `encryption_key_id` para trackear qué clave se usó (soporte rotación)
- Interceptor en secrets store: al hacer Set, encrypt antes de INSERT; al hacer Get, decrypt después de SELECT
- Comando CLI `domain secrets re-encrypt` para rotación de clave
- Secrets store con tabla `secrets` (id, organization_id, name, encrypted_value, encryption_algorithm, encryption_key_id, created_at, updated_at)

**No incluye:**
- Encriptación de otros campos (solo secrets)
- HSM externo o KMS
- Encriptación a nivel de columna en Postgres (pgcrypto)

## Enfoque técnico

1. AES-256-GCM con 12-byte nonce aleatorio + cifrado autenticado (AEAD)
2. Output format: nonce (12) + ciphertext (variable) + auth tag (16)
3. Storage: Base64 URL-safe sin padding
4. Encryption key: si DOMAIN_ENCRYPTION_KEY tiene ≠ 32 bytes, aplicar SHA-256 para obtener 32 bytes
5. `encryption_key_id`: hash SHA-256 de la clave de encriptación (para identificar qué clave se usó)
6. Re-encrypt: iterar secrets, decrypt con key anterior, encrypt con key nueva, update en transacción

## Riesgos

- **Pérdida de clave de encriptación:** Los secrets son irrecuperables. Mitigación: documentar claramente, recomendar backup de DOMAIN_ENCRYPTION_KEY.
- **Nonce reutilizado:** AES-256-GCM con nonce repetido + misma clave = catastrophic failure. Mitigación: crypto/rand es seguro, nonce de 12 bytes es suficientemente grande.
- **Clave en logs:** Nunca loguear DOMAIN_ENCRYPTION_KEY ni secrets desencriptados.
- **Rotación parcial:** Si el proceso de re-encrypt se interrumpe, algunos secrets quedan con clave vieja. Mitigación: transacción por secret, re-ejecutable.

## Testing

- Test encrypt/decrypt roundtrip
- Test nonce uniqueness (1000 encrypt del mismo valor, todos distintos)
- Test decrypt con diferente clave falla
- Test decrypt con ciphertext corrupto falla
- Test re-encrypt de todos los secrets
- Test que el store llama a encrypt/decrypt automáticamente
- Test key derivation (SHA-256 cuando length != 32)
