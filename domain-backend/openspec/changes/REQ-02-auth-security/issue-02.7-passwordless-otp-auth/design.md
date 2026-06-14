# Design: issue-02.7-passwordless-otp-auth

## Decisión arquitectónica

**Credential modelo:** API key como único token de acceso (no sesiones server-side ni cookies).
**OTP:** 6 dígitos numéricos, bcrypt cost 10, TTL 10min, single-use, max 5 attempts.
**RUT:** formato canónico `NNNNNNNN-X` (sin puntos), `K` siempre mayúscula, validación módulo 11.
**Hash de API key:** bcrypt cost 12 (lookup cuesta ~250ms; aceptable: solo en login).
**Rate limiting:** tabla `auth_rate_limits` particionada por hora (cron drop partition viejo).

## Alternativas descartadas

- **Sesiones server-side con cookies**: descartado — el user pidió "JSON simple con la apikey"; mantiene flow CLI-friendly y stateless
- **JWT como API key**: revocación instantánea requiere blacklist; bcrypt-hash en DB es más directo
- **Magic link en lugar de OTP**: link más cómodo pero pega worse en CLI y phishing-friendly; OTP de 6 dígitos es estándar (Stripe, Vercel, etc.)
- **TOTP (authenticator app)**: requiere onboarding extra; OTP por email es suficiente para MVP B2B con invitación admin
- **SMS OTP**: costo, complejidad regional, deliverability peor que email

## Schema

```sql
-- Migración: agregar columnas a users
ALTER TABLE users
  ADD COLUMN rut VARCHAR(12) UNIQUE,
  ADD COLUMN last_organization_id UUID REFERENCES organizations(id),
  ADD COLUMN last_login_at TIMESTAMPTZ;

CREATE INDEX ON users (rut) WHERE rut IS NOT NULL AND deleted_at IS NULL;

-- Nueva tabla OTP
CREATE TABLE otp_codes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  code_hash BYTEA NOT NULL,            -- bcrypt cost 10
  attempts SMALLINT NOT NULL DEFAULT 0,
  max_attempts SMALLINT NOT NULL DEFAULT 5,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  ip_address VARCHAR(45),
  user_agent VARCHAR(500),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ON otp_codes (user_id, created_at DESC)
  WHERE used_at IS NULL;

-- Tabla rate limiting (particionada por hora)
CREATE TABLE auth_rate_limits (
  bucket VARCHAR(80) NOT NULL,   -- "otp:identifier:bob@x.com" | "otp:ip:1.2.3.4"
  window_start TIMESTAMPTZ NOT NULL,
  count INT NOT NULL DEFAULT 0,
  PRIMARY KEY (bucket, window_start)
) PARTITION BY RANGE (window_start);

-- API keys: reusa issue-02.1; requerimiento adicional aquí:
-- "una sola activa por user" se implementa en service, no constraint
-- porque la tabla api_keys permite múltiples para admin/service
```

## Endpoints

| método | path | auth | descripción |
|--------|------|------|-------------|
| POST | /auth/request-otp | público | recibe `{identifier}` envía OTP al email |
| POST | /auth/verify-otp | público | recibe `{identifier, code, action}` devuelve `{api_key, ...}` |
| GET | /me | api_key | devuelve user + orgs + key_prefix actual |
| POST | /me/api-key/revoke | api_key | revoca la propia API key actual (logout-like) |

## Bodies

**Request OTP**
```json
POST /auth/request-otp
{ "identifier": "bob@x.com" }   // o "12.345.678-5" / "12345678-5" / "123456785"
→ 200 { "sent": true, "expires_in_seconds": 600 }
```

**Verify OTP**
```json
POST /auth/verify-otp
{ "identifier": "bob@x.com", "code": "482917", "action": "reveal" | "regenerate" }
→ 200 {
  "api_key": "domk_live_<...>",
  "key_prefix": "domk_live_a1b2c3",
  "created_at": "2026-06-06T12:00:00Z",
  "regenerated": false,
  "is_first": false,
  "user": { "id": "...", "email": "bob@x.com", "rut": "12345678-5", "name": "Bob" },
  "organizations": [ { "id": "...", "slug": "acme", "role": "member" } ]
}
```

## RUT normalization y validación

```go
// internal/auth/rut/rut.go
// Normalize quita puntos, asegura guión, K mayúscula.
// "12.345.678-5" → "12345678-5"
// "123456785"    → "12345678-5"
// "12345678k"    → "12345678-K"
func Normalize(raw string) (string, error)

// Validate confirma dígito verificador con módulo 11 (pesos 2..7 cíclicos)
func Validate(normalized string) bool
```

## Flow request-otp

```
1. Parse body { identifier }
2. Rate limit checks:
   - identifier: ≤5/h, ≤1/60s
   - IP: ≤10/h
   - si excede: 429 con retry_after
3. Detect identifier kind:
   - tiene "@" → email
   - else → try RUT normalize+validate
     - si inválido → simular happy path (sleep ~200ms), return 200 fake
4. Find user by email or rut, WHERE deleted_at IS NULL AND organization.deleted_at IS NULL
   - si no existe → simular happy path, return 200 fake
5. Si existe:
   - generar code [crypto/rand entre 100000 y 999999]
   - bcrypt hash
   - INSERT otp_codes (expires_at = now()+10min)
   - notifications.Enqueue("otp_email", { to: user.email, code, expires_in: "10 minutos" })
6. Return 200 { sent: true, expires_in_seconds: 600 }
```

## Flow verify-otp

```
1. Parse body { identifier, code, action }
2. Rate limit similar (más estricto en verify: 10/h por identifier)
3. Detect identifier → find user (mismo que request-otp)
   - si no existe → respuesta uniforme 401 "invalid_code" (timing constante)
4. SELECT FOR UPDATE último otp_codes del user WHERE used_at IS NULL ORDER BY created_at DESC LIMIT 1
   - si no hay → 410 "no_active_otp"
   - si expires_at < now() → 410 "otp_expired"
   - si attempts >= max → 429 "too_many_attempts"
5. bcrypt.CompareHashAndPassword(code_hash, code)
   - si mismatch:
     - UPDATE attempts++
     - return 401 "invalid_code" attempts_remaining
   - si match:
     - UPDATE used_at = now()
     - SELECT FOR UPDATE api_keys del user WHERE revoked_at IS NULL
     - si action="regenerate" o no hay key activa:
       - UPDATE prev keys SET revoked_at = now()
       - generar new key (32 bytes base64url)
       - INSERT api_keys (key_hash bcrypt cost 12, key_prefix, name="user-primary")
       - regenerated = (had_previous_active)
     - else action="reveal":
       - retrieve existing key (NO podemos exponer el secret completo si solo guardamos hash)
       - ⚠️ Implicación: en "reveal" el secret real NO está disponible. Decisión:
         - Almacenar adicionalmente cifrado simétrico (AES-256-GCM con master key del server, issue-02.3) y descifrar solo en reveal
6. audit_log "auth.otp_verified" y "api_key.revealed|rotated"
7. UPDATE users SET last_login_at = now()
8. Return 200 con JSON
```

## ⚠️ Trade-off: "reveal" requiere almacenar la key recuperable

Una API key normal se hashea irreversiblemente. Para que el flow "reveal" devuelva la key actual (no solo una nueva), necesitamos almacenarla recuperablemente:

**Opción A — Cifrado simétrico (recomendada):**
- Campo nuevo `api_keys.key_encrypted BYTEA` con AES-256-GCM usando master key del issue-02.3
- Key plaintext NUNCA en logs ni response excepto en /auth/verify-otp
- Master key rotable (re-encrypt all on rotation)
- Hash bcrypt se mantiene para auth check fast

**Opción B — Solo rotate:**
- "reveal" en realidad siempre regenera
- Etiquetar action="reveal" como deprecated y forzar regenerate
- Simpler pero rompe expectativa del user ("usar la anterior")

**Decisión:** Opción A para respetar requerimiento del usuario.

## Componentes

```
internal/auth/otp/
  service.go        # Request / Verify orchestrators
  generator.go      # crypto/rand 6-digit code
  store.go          # CRUD otp_codes
  ratelimit.go      # bucket Postgres
internal/auth/rut/
  rut.go            # Normalize, Validate
  rut_test.go
internal/auth/apikey/
  generator.go      # 32 bytes b64url
  store.go          # CRUD api_keys con encrypt/decrypt
  encryption.go     # AES-256-GCM wrap
internal/http/handlers/
  auth.go           # request-otp, verify-otp
  me.go             # GET /me, revoke
```

## Templates de notificación

```
slug: otp_email
subject: "Tu código de acceso a Domain: {{.Code}}"
body_text: |
  Hola,
  Tu código de acceso es: {{.Code}}
  Expira en {{.ExpiresIn}}.
  Si no fuiste vos, ignorá este mensaje.
body_html: <multipart htmlversion>
variables: [Code, ExpiresIn, UserAgent, IPAddress]
```

## TDD plan

1. **RUT**: tabla de fixtures con RUTs válidos/inválidos → Normalize + Validate
2. **OTP generator**: 1000 codes → todos 6 dígitos, distribución uniforme estadística
3. **Request OTP happy path** email → Mailpit recibe + tabla otp_codes nueva row
4. **Request OTP RUT formatos**: 3 formatos → match mismo user
5. **Request OTP user inexistente**: response 200 fake, sin email
6. **Request OTP RUT DV inválido**: response 200 fake, sin email
7. **Request OTP rate limit identifier**: 6ta solicitud en 1h → 429
8. **Verify happy path reveal**: API key actual returned (descifrada)
9. **Verify happy path regenerate**: nueva key, vieja revocada
10. **Verify primer login** (sin key previa): nueva key + is_first true
11. **Verify code incorrecto 5x**: 401 attempts_remaining decreciente; 6ta → 429 too_many_attempts y OTP marcado expired
12. **Verify OTP expirado**: 410
13. **Verify OTP ya usado**: 410
14. **Race regenerate paralelo**: 2 verify-otp action=regenerate → solo 1 API key activa al final (FOR UPDATE)
15. **Encryption rotation**: rotar master key → all keys re-encrypt funcionan
16. **Timing constante**: medir 1000 request-otp con/sin user → p99 dentro de ±5ms
17. **Sabotaje**: code stored in plaintext (no bcrypt) → linter test detecta
18. **Sabotaje**: log line incluye `code=...` → linter PII falla

## Riesgos y mitigación

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|-------------|---------|------------|
| Email no llega | Media | Alto | Rate limit permite reenvío c/60s; status check Mailpit dev |
| Enumeration usuarios | Alta | Medio | Response uniforme + timing constante + tests p99 |
| Brute-force OTP | Media | Alto | bcrypt + max 5 attempts + rate limit IP |
| Inbox compromise | Baja | Crítico | Aceptado: misma exposure que cualquier passwordless; invitación admin reduce surface |
| Master key (encryption) leak | Baja | Crítico | Key en KMS/Vault; rotación periódica documentada |
| Race rotate | Media | Medio | SELECT FOR UPDATE en api_keys + tx única |
| RUT inválido aceptado | Baja | Bajo | DV validate + tests con casos reales chilenos |
