# Design: issue-02.3-secrets-encryption

## Decisión arquitectónica

**Algoritmo:** AES-256-GCM (cifrado autenticado con datos asociados)
**Nonce:** 12 bytes, generado con `crypto/rand` por operación
**Formato almacenado:** Base64(nonce || ciphertext || auth_tag)
**Key derivation:** Si la clave no tiene exactamente 32 bytes, aplicar SHA-256 para normalizar
**Capa de integración:** Secrets store wrapper que encripta antes de INSERT y desencripta después de SELECT

## Alternativas descartadas

- **pgcrypto:** Encriptación a nivel de BD, no da control sobre nonce ni key rotation desde Go.
- **AWS KMS / GCP Cloud KMS:** Dependencia externa que no queremos en esta etapa.
- **AES-256-CBC:** No provee autenticación (MAC), vulnerable a padding oracle attacks.
- **ChaCha20-Poly1305:** Alternativa válida pero menos soporte en hardware. AES-256-GCM es más estándar.

## Diagrama

```
Encrypt:
  plaintext → AES-256-GCM Encrypt(nonce, key, plaintext)
              → nonce(12) || ciphertext || tag(16)
              → Base64 encode
              → almacenar en encrypted_value

Decrypt:
  encrypted_value → Base64 decode
                  → split nonce(12) || ciphertext+tag
                  → AES-256-GCM Decrypt(nonce, key, ciphertext+tag)
                  → plaintext

Store flow:
  Set(name, plaintext):
    encrypted = Encrypt(plaintext)
    INSERT INTO secrets (name, encrypted_value, encryption_algorithm, encryption_key_id, ...)

  Get(name):
    row = SELECT encrypted_value, encryption_algorithm, encryption_key_id FROM secrets WHERE name = ?
    plaintext = Decrypt(row.encrypted_value)
    return plaintext

Key Rotation:
  newKey = load new DOMAIN_ENCRYPTION_KEY
  oldKey = key anterior (guardada temporalmente)
  FOR each secret:
    plaintext = Decrypt(oldKey, secret.encrypted_value)
    newEncrypted = Encrypt(newKey, plaintext)
    UPDATE secret SET encrypted_value = newEncrypted, encryption_key_id = newKeyID
```

## Secrets table schema

```sql
CREATE TABLE secrets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    encrypted_value TEXT NOT NULL,
    encryption_algorithm VARCHAR(50) NOT NULL DEFAULT 'aes-256-gcm',
    encryption_key_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(organization_id, name)
);
```

## Cipher interface

```go
type Cipher interface {
    Encrypt(plaintext []byte) (ciphertext []byte, err error)
    Decrypt(ciphertext []byte) (plaintext []byte, err error)
}

type AESGCMCipher struct {
    key   []byte // 32 bytes
    keyID string // SHA-256 hash of key
}

func NewAESGCMCipher(encryptionKey string) (*AESGCMCipher, error) {
    key := deriveKey(encryptionKey) // SHA-256 if not 32 bytes
    keyID := sha256Hex(key)
    return &AESGCMCipher{key: key, keyID: keyID}, nil
}
```

## TDD plan

1. Test encrypt/decrypt roundtrip con texto conocido
2. Test nonce uniqueness (1000 iteraciones)
3. Test decrypt con diferente key → error
4. Test decrypt con ciphertext corrupto → error
5. Test key derivation con key de 16 bytes → resulta en 32 bytes
6. Test key derivation con key de 32 bytes → mismo resultado (sin hash)
7. Test store.Set encripta antes de INSERT
8. Test store.Get desencripta después de SELECT
9. Test re-encrypt CLI command
10. Test que encryption_key_id cambia después de re-encrypt

## Riesgos y mitigación

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|-------------|---------|------------|
| Pérdida de encryption key | Baja | Crítico | Documentar backup; mostrar key_id en health check |
| Nonce colisión | Extremadamente baja | Crítico | crypto/rand con 12 bytes (96 bits) |
| Re-encrypt incompleto | Baja | Medio | Transacción por secret; re-ejecutable |
| Log accidental de secrets | Media | Alto | Sanitizer en logger; no loguear bodies de secrets endpoints |
