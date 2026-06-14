# Proposal: issue-02.1-api-key-auth

## Intención

Implementar autenticación por API keys seguras: generación criptográfica con prefijo, hashing bcrypt, middleware de validación, CRUD completo, rotación y revocación.

## Scope

**Incluye:**
- Generación de key: 32 bytes aleatorios de `crypto/rand`, encode Base64URL, prefijo `mem_`
- Hashing: bcrypt con costo 12
- Almacenamiento: `api_keys` table (key_hash VARCHAR(255), key_prefix VARCHAR(10), name, organization_id, user_id, expires_at, revoked_at)
- Middleware `AuthMiddleware` que lee `X-API-Key`, busca por key_prefix, verifica bcrypt, chequea expiración/revocación
- Endpoints CRUD:
  - `POST /api/v1/api-keys` — generar
  - `GET /api/v1/api-keys` — listar (sin hash)
  - `DELETE /api/v1/api-keys/:id` — borrar
  - `POST /api/v1/api-keys/:id/rotate` — rotar
  - `POST /api/v1/api-keys/:id/revoke` — revocar
- Key original retornada solo en creación y rotación (una vez)
- Request context enriquecido: `api_key_id`, `organization_id`, `user_id`

**No incluye:**
- Rate limiting (issue-02.5)
- RBAC (issue-02.2)
- API keys con scopes específicos (v2)

## Enfoque técnico

1. `crypto/rand` para generación de bytes aleatorios
2. `golang.org/x/crypto/bcrypt` para hashing con costo 12
3. key_prefix: primeros 8 chars de la key original (para lookup rápido sin escanear todos los hashes)
4. Validación en middleware: lookup por key_prefix → bcrypt.Compare → check revoked_at/expires_at
5. Rotación: nueva key, nuevo hash, key anterior marcada como revoked_at = now()
6. Endpoints protegidos por el propio middleware (requieren API key vigente para gestionar keys)

## Riesgos

- **bcrypt costo alto:** Costo 12 puede ser lento (~250ms). Mitigación: hacer configurable via config; considerar cache de keys válidas en memoria.
- **Key prefix collision:** 8 chars Base64URL = 48 bits de entropía. Colisión improbable. Mitigación: si hay colisión, regenerar prefix.
- **Key original en logs:** Posible leak si se loguea el request body. Mitigación: sanitizar antes de loggear, no incluir key en responses de listado.
- **Timing attack en lookup por prefix:** Mínimo. El atacante ya tiene que tener acceso a la DB para explotarlo.

## Testing

- Test generación de key cumple formato
- Test bcrypt hash/verify
- Test middleware con key válida, inválida, expirada, revocada
- Test CRUD endpoints
- Test rotación: key anterior revocada, nueva key activa
- Test que key original solo se retorna en creación/rotación
