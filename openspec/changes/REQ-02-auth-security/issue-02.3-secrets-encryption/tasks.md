# Tasks: issue-02.3-secrets-encryption

## Backend

- [x] Crear cipher AES-256-GCM → internal/crypto/aesgcm.go (keyring multi-versión)
- [x] Implementar constructor con key derivation → NewCipher + crypto.DeriveKey (SHA-256 para inputs ≠32 bytes) — 2026-06-10
- [x] Implementar formato [version|nonce(12)|ciphertext+tag] → aesgcm.go (version inline para rotation)
- [x] Crear `internal/secrets/store.go` con PGStore Postgres
- [x] Integrar encrypt antes de INSERT → store.Create
- [x] Integrar decrypt después de SELECT → store.GetValue
- [x] Agregar columna de versión de key → encryption_key_version (migration 000019)
- [x] Implementar re-encrypt → PGStore.ReEncryptAll (idempotente; usa pools.Auth BYPASSRLS por ser batch cross-org) — 2026-06-10
- [x] Agregar comando CLI `domain secrets re-encrypt` → runSecretsCmd con DOMAIN_MASTER_KEYS="1:<b64>,2:<b64>" (crypto.LoadKeyring) — 2026-06-10
- [x] Validar en startup que la master key tenga 32 bytes → ErrInvalidKeySize en NewCipher/LoadKeyring
- [x] Agregar endpoint CRUD para secrets (protegido con RBAC) → handler.API.SecretsStore

## Tests

- [x] Test unitario: encrypt/decrypt roundtrip → TestCipher_RoundTrip
- [x] Test unitario: nonce uniqueness (1000 veces) → TestEncrypt_NonceUnique_1000x — 2026-06-10
- [x] Test unitario: decrypt con key diferente falla → TestSabotage_WrongKey_Rejected
- [x] Test unitario: decrypt con ciphertext corrupto falla → TestSabotage_TamperedCiphertext_Rejected
- [x] Test unitario: key derivation 16→32 bytes → TestDeriveKey_ShortInputDerivesTo32 — 2026-06-10
- [x] Test unitario: key derivation con 32 bytes = sin cambios → TestDeriveKey_Exact32Unchanged — 2026-06-10
- [x] Test unitario: store.Create encripta → TestReEncryptAll_RotatesToCurrentVersion (Create asigna versión + valor cifrado en DB) — 2026-06-10
- [x] Test unitario: store.GetValue desencripta → mismo test (valores legibles post-rotation) — 2026-06-10
- [x] Test unitario: re-encrypt cambia encryption_key_version → TestReEncryptAll_RotatesToCurrentVersion (v1→v2 + idempotencia) — 2026-06-10
- [x] Sabotaje: keyring sin la versión del blob → ErrUnknownKeyVersion → TestLoadKeyring_MultiVersionRoundTrip — 2026-06-10
- [x] Sabotaje: specs de keyring inválidos rechazados → TestLoadKeyring_InvalidSpecs — 2026-06-10

## Cierre

- [x] Verificación manual: crear secret, leerlo, verificar cifrado en DB → cubierto E2E por TestReEncryptAll_RotatesToCurrentVersion (testcontainers, valor nunca en claro)
- [x] Suite verde → 2026-06-10: 19 tests crypto + integración secrets verde
