# Proposal: HU-02.7-passwordless-otp-auth

## Intención

Login passwordless por OTP (one-time password) enviado al email del usuario. Identifier puede ser RUT chileno (con validación módulo 11) o email. Al validar el código, la respuesta es un JSON simple con la API key del usuario (revelar la actual o regenerar). Sin self-signup: solo usuarios previamente invitados pueden loguearse.

## Scope

**Incluye:**
- Tabla `otp_codes` con hash bcrypt, expiración 10min, max 5 attempts, single-use
- Columna `users.rut VARCHAR(12) UNIQUE` normalizada (formato `12345678-5`)
- Endpoints REST: `POST /auth/request-otp`, `POST /auth/verify-otp`
- Endpoint `GET /me` (autenticado con API key)
- Normalizador y validador de RUT (módulo 11)
- Integración con canal `email-smtp` (HU-20.2) para envío de código
- Generación/rotación de API key (integra HU-02.1) con regla "una activa por user"
- Rate limiting por identifier (5/h) y por IP (10/h)
- Anti-enumeration: respuesta 200 idéntica aunque identifier no exista
- Audit log de request/verify/regenerate

**No incluye:**
- Self-signup público (decidido: solo invitación admin)
- Sesiones server-side / cookies (la API key es el único credential)
- SSO con otros providers (futuro como HU separadas si se necesita)
- SMS / WhatsApp OTP (solo email en MVP)
- Multi-factor en steps adicionales (OTP por email YA es el 2do factor sobre "saber email")

## Enfoque técnico

1. **OTP**: 6 dígitos numéricos generados con `crypto/rand`; hash con bcrypt cost 10 antes de persistir
2. **RUT**: normalización (strip puntos, lowercase K, format `NNNNNNNN-X`) + validación módulo 11 con tabla de pesos `[2,3,4,5,6,7]`
3. **Anti-enumeration**:
   - Timing constante (medir promedio del happy path y `time.Sleep` para uniformar fast paths)
   - Response 200 idéntico aunque el user no exista, sin email enviado
   - Logs internos diferenciados (no expuestos al cliente)
4. **API key**:
   - Format `domk_live_<32 bytes base64url>` (prefijo identifica entorno)
   - `key_prefix` = primeros 16 chars para identificar sin exponer secret
   - `key_hash` = bcrypt cost 12 del key completo
   - Regla "una activa por user": antes de generar nueva, revocar las anteriores
5. **Rate limit**: token bucket en Postgres `auth_rate_limits` o en memoria con expiración (Postgres + cron de cleanup)
6. **Email**: enqueue via `notifications.Enqueue("otp_email", ...)` con template `otp_email`

## Riesgos

- **Enumeration de usuarios**: timing constante + response idéntico + logs no expuestos
- **Brute-force OTP**: max 5 attempts + bcrypt hash (constant-time compare)
- **Replay del email**: single-use (`used_at`) + expira 10min
- **Email no llega**: rate limit permite reenvío cada 60s; mostrar en UI
- **Race en rotate**: `SELECT FOR UPDATE` sobre `api_keys` del user antes de revocar + insertar nueva en misma tx
- **Email del invitado fue capturado** (control compromise): mismo problema que cualquier passwordless; mitigación = invitaciones expiran rápido (HU-21.2 ya cubre)
- **RUT collision**: UNIQUE constraint a nivel DB
- **Rate limit memory leak**: Postgres-backed con cron expiración

## Testing

- Happy path completo: request → recibir email (Mailpit dev) → verify → JSON con API key
- Login por email y por RUT (con y sin puntos)
- RUT inválido (dígito incorrecto, formato malformado) → fake 200 sin email
- Identifier inexistente → fake 200 sin email
- OTP incorrecto 5 veces → 429 "too_many_attempts"
- OTP expirado → 410
- OTP ya usado → 410
- Rate limit identifier 60s → 429
- Rate limit IP 10/h → 429
- Regenerate: API key vieja queda revocada, no funciona
- Reveal: API key actual devuelta sin rotar
- User sin API key previa (primer login) → genera nueva con `is_first: true`
- User suspendido → fake 200 sin email
- Race: 2 verify-otp regenerate concurrentes → solo una API key activa al final
