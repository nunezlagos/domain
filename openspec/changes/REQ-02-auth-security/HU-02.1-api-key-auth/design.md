# Design: HU-02.1-api-key-auth

## Decisión arquitectónica

**Generación:** `crypto/rand` (32 bytes) → Base64URL sin padding → prefijo `mem_`
**Hashing:** bcrypt con costo 12 (`golang.org/x/crypto/bcrypt`)
**Lookup strategy:** key_prefix (primeros 8 chars) + bcrypt.Compare
**Middleware:** HTTP middleware que inyecta contexto con `APIKeyID`, `OrganizationID`, `UserID`
**Store:** `internal/auth/store.go` con métodos CRUD sobre tabla `api_keys`

## Alternativas descartadas

- **SHA-256 + salt en DB sin bcrypt:** Menos resistencia a brute force si hay leak de DB. bcrypt es más costoso deliberadamente.
- **JWT como API keys:** Los JWT pueden ser inspeccionados (payload visible). Preferimos opaque tokens.
- **Hash de toda la key para lookup:** Haría lookup O(n) sobre todos los hashes. Prefix permite búsqueda eficiente.

## Diagrama

```
Generación:
  crypto/rand(32) → base64RawURL → "mem_" + raw
                                 → key_prefix = raw[:8]
                                 → bcrypt_hash = bcrypt(raw, cost=12)
                                 → INSERT (key_hash, key_prefix, ...)
                                 → return raw key (una vez)

Autenticación:
  Request → X-API-Key: mem_abc123...
                │
                └─→ Extract raw key, prefix = raw[:8]
                └─→ SELECT * FROM api_keys WHERE key_prefix = prefix AND revoked_at IS NULL
                └─→ bcrypt.Compare(key_hash, raw_key)
                └─→ Check expires_at > now()
                └─→ Inyectar contexto: { api_key_id, organization_id, user_id }
                └─→ Next handler

Middleware chain:
  Router → AuthMiddleware → [context enriched] → Handler
```

## Store interface

```go
type APIKeyStore interface {
    Create(ctx context.Context, orgID, userID uuid.UUID, name string, expiresAt *time.Time) (*APIKey, string, error)
    GetByID(ctx context.Context, id uuid.UUID) (*APIKey, error)
    ListByOrganization(ctx context.Context, orgID uuid.UUID) ([]*APIKey, error)
    Delete(ctx context.Context, id uuid.UUID) error
    Rotate(ctx context.Context, id uuid.UUID) (*APIKey, string, error)
    Revoke(ctx context.Context, id uuid.UUID) error
    Authenticate(ctx context.Context, rawKey string) (*APIKey, error)
}
```

## TDD plan

1. Test generar key: formato `mem_`, largo ≥ 40, key_prefix correcto
2. Test hash/verify bcrypt
3. Test Authenticate con key válida
4. Test Authenticate con key inválida (hash no coincide)
5. Test Authenticate con key expirada
6. Test Authenticate con key revocada
7. Test CreateAndReturn: key original retornada una vez
8. Test Rotate: nueva key, anterior revocada
9. Test ListByOrganization: sin key_hash ni key original
10. Test Delete: key eliminada físicamente
11. Test middleware 401 en cada caso de error

## Riesgos y mitigación

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|-------------|---------|------------|
| bcrypt lento en alta concurrencia | Media | Alto | Hacer costo configurable; cache de keys válidas (TTL 5min) |
| Key original en access logs | Media | Alto | Sanitizer middleware; no loguear headers de auth |
| Key prefix collision | Muy baja | Alto | Re-generar si hay colisión |
| Perder key original sin haberla guardado | Baja | Medio | Es por diseño — la key se muestra una vez y es responsabilidad del usuario guardarla |
