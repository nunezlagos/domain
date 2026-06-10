# Tasks: issue-02.7-passwordless-otp-auth

## Schema

- [ ] **db-001**: Migración `ALTER TABLE users ADD COLUMN rut, last_organization_id, last_login_at`
- [ ] **db-002**: Migración `CREATE TABLE otp_codes` + índices
- [ ] **db-003**: Migración `CREATE TABLE auth_rate_limits` particionada por hora
- [ ] **db-004**: Migración `ALTER TABLE api_keys ADD COLUMN key_encrypted BYTEA` (para reveal)
- [ ] **db-005**: Cron drop partitions de `auth_rate_limits` >7 días

## RUT

- [ ] **rut-001**: `internal/auth/rut/rut.go` Normalize (puntos, guión, K)
- [ ] **rut-002**: `rut.go` Validate dígito verificador módulo 11
- [ ] **rut-003**: Tabla fixtures `rut_test.go` con válidos/inválidos chilenos

## OTP

- [ ] **otp-001**: `internal/auth/otp/generator.go` con crypto/rand 6 dígitos
- [ ] **otp-002**: `otp/store.go` CRUD bcrypt cost 10
- [ ] **otp-003**: `otp/ratelimit.go` token bucket Postgres
- [ ] **otp-004**: `otp/service.go` Request orchestrator
- [ ] **otp-005**: `otp/service.go` Verify orchestrator con FOR UPDATE
- [ ] **otp-006**: Timing-constant comparator para identifier-not-found

## API key

- [ ] **ak-001**: `internal/auth/apikey/generator.go` 32 bytes base64url + prefix `domk_live_`
- [ ] **ak-002**: `apikey/encryption.go` AES-256-GCM wrap/unwrap con master key
- [ ] **ak-003**: `apikey/store.go` Generate (revoca previas) + Reveal (decrypt)
- [ ] **ak-004**: Integración issue-02.3 secrets para master key
- [ ] **ak-005**: Hook rotación master key → re-encrypt all keys

## HTTP

- [ ] **http-001**: `internal/http/handlers/auth.go` POST /auth/request-otp
- [ ] **http-002**: `auth.go` POST /auth/verify-otp
- [ ] **http-003**: `handlers/me.go` GET /me con user + orgs + key_prefix
- [ ] **http-004**: `me.go` POST /me/api-key/revoke
- [ ] **http-005**: Anti-enumeration timing wrapper

## Notifications

- [ ] **notif-001**: Template `otp_email` HTML + text en migraciones seed
- [ ] **notif-002**: Wire `notifications.Enqueue("otp_email", ...)` desde Request

## Config

- [ ] **cfg-001**: Vars en issue-01.2: `DOMAIN_OTP_TTL_SECONDS=600`, `DOMAIN_OTP_LENGTH=6`, `DOMAIN_OTP_MAX_ATTEMPTS=5`
- [ ] **cfg-002**: `DOMAIN_OTP_RATE_LIMIT_PER_IDENTIFIER_HOUR=5`, `DOMAIN_OTP_RATE_LIMIT_PER_IP_HOUR=10`
- [ ] **cfg-003**: `DOMAIN_OTP_RESEND_COOLDOWN_SECONDS=60`
- [ ] **cfg-004**: `DOMAIN_APIKEY_PREFIX=domk_live_`

## Tests

- [ ] **test-001**: RUT formats normalizan a canónico
- [ ] **test-002**: RUT DV inválidos detectados
- [ ] **test-003**: OTP 6 dígitos crypto/rand distribución
- [ ] **test-004**: Happy path email → Mailpit dev
- [ ] **test-005**: Happy path RUT (3 formatos)
- [ ] **test-006**: User inexistente → fake 200, sin email
- [ ] **test-007**: RUT DV inválido → fake 200, sin email
- [ ] **test-008**: Verify reveal API key actual
- [ ] **test-009**: Verify regenerate rota
- [ ] **test-010**: Primer login → is_first true
- [ ] **test-011**: 5x code incorrecto → attempts decreciente; 6ta → 429
- [ ] **test-012**: OTP expirado → 410
- [ ] **test-013**: OTP ya usado → 410
- [ ] **test-014**: Rate limit identifier 6ta/h → 429
- [ ] **test-015**: Rate limit IP 11va/h → 429
- [ ] **test-016**: Resend cooldown <60s → 429 retry_after
- [ ] **test-017**: User suspendido → fake 200
- [ ] **test-018**: Race regenerate concurrente → 1 key activa
- [ ] **test-019**: Encryption master key rotation → all keys still revealed
- [ ] **test-020**: Timing constante p99 dentro de ±5ms
- [ ] **sabotaje-001**: log line con code= → linter PII falla
- [ ] **sabotaje-002**: store code plaintext (sin bcrypt) → linter falla

## Docs

- [ ] **docs-001**: `docs/auth/otp-login.md` con curl ejemplos + JSON responses
- [ ] **docs-002**: `docs/auth/api-keys.md` con formato, regeneración, rotación
- [ ] **docs-003**: Ejemplo "Web simple" en `docs/ui/login-page.md` (form 2 steps + JSON viewer)

## Cierre

- [ ] Smoke end-to-end en dev compose: request-otp → ver código en Mailpit → verify-otp → recibir JSON con API key
- [ ] Probar con RUT real chileno (uno propio dev)
