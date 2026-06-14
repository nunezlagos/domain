# Tasks: issue-32.1-session-web-auth-cookies-jwt

## Backend

- [ ] **T1**: Crear migración `migrations/000091_web_sessions.sql`:
  - Tabla `web_sessions` con schema de design.md.
  - 2 índices (user, refresh_token).
  - Constraints: `expires_at > created_at`, `revoked_at IS NULL
    OR revoked_at >= created_at`.

- [ ] **T2**: Crear paquete `internal/auth/websession/` con:
  - `service.go` — `type Service struct { Pool, Cipher, Logger }`.
    Métodos: `Create`, `Refresh`, `Revoke`, `RevokeAll`, `GetByID`.
  - `middleware.go` — `Middleware` que extrae session_id de
    `Cookie: session_id=...` O de `Authorization: Bearer <jwt>`.
  - `csrf.go` — `CSRFToken()` helper que genera random 32 bytes
    base64, lo setea en cookie `csrf_token` (no httpOnly) si no
    existe.

- [ ] **T3**: Crear `internal/auth/jwt/jwt.go`:
  - `Sign(claims jwt.MapClaims, key []byte) (string, error)`.
  - `Verify(tokenStr string, key []byte) (jwt.MapClaims, error)`.
  - Usa `golang-jwt/jwt/v5`.
  - Soporta keyring multi-versión (verify con la más alta; signing
    con la más alta).

- [ ] **T4**: Handler `internal/api/handler/auth_web.go`:
  - `LoginPOST(w, r)` — body `{email}` → invoca `otp.RequestOTP`,
    retorna 202.
  - `VerifyPOST(w, r)` — body `{email, otp_code}` → valida OTP,
    crea sesión, retorna Set-Cookie + body con `expires_at`.
  - `RefreshPOST(w, r)` — body `{refresh_token}` o cookie →
    emite nuevos tokens.
  - `LogoutPOST(w, r)` — revoca sesión, limpia cookie.
  - `LogoutAllPOST(w, r)` — revoca todas del user.
  - `MeGET(w, r)` — retorna info del user.

- [ ] **T5**: Middleware de auth dual en
  `cmd/domain/main.go` (reemplaza el `authMW` actual):
  - Detecta `Authorization: Bearer sk_*` → `apikey.Middleware`.
  - Detecta `Cookie: session_id=*` o `Authorization: Bearer
    <jwt>` → `websession.Middleware`.
  - Si ninguno: 401 (excepto rutas en allowlist).

- [ ] **T6**: CSRF middleware en `internal/api/middleware/csrf.go`:
  - Métodos seguros (GET, HEAD, OPTIONS): skip.
  - Mutaciones: require header `X-CSRF-Token` matching cookie
    `csrf_token` (constant-time compare).
  - 403 si falta o no matchea.

- [ ] **T7**: Config: agregar a `config.Config`:
  - `SessionTTLHours int` (default 168).
  - `AccessTokenTTLMinutes int` (default 60).
  - `CookieSecure bool` (default true).
  - `CookieDomain string` (opcional).

- [ ] **T8**: Allowlist de rutas públicas en
  `handler.AuthAllowlist()`: agregar `/auth/login`, `/auth/verify`,
  `/auth/refresh` (con rate limit), `/health`, `/api/version`,
  `/api/v1/openapi.json` (de 32.3).

- [ ] **T9**: Job nightly de purga de sesiones: en
  `runSoftDeletePurge` o similar, agregar
  `DELETE FROM web_sessions WHERE (revoked_at IS NOT NULL AND
  revoked_at < NOW() - INTERVAL '30 days') OR (expires_at < NOW()
  - INTERVAL '30 days')`. Retorna count.

## Tests

- [ ] **T-unit-1**: `TestService_Create_StoresHash**` — Create
  retorna tokens; el hash del refresh está en DB (no el
  plaintext).
- [ ] **T-unit-2**: `TestService_Refresh_RotatesTokens**` —
  Refresh emite access nuevo; el refresh viejo queda revoked.
- [ ] **T-unit-3**: `TestService_Revoke_InvalidatesSession**` —
  post-Revoke, `GetByID` retorna error "session revoked".
- [ ] **T-unit-4**: `TestService_RevokeAll_KillsAllUserSessions**`
  — user con 3 sesiones, RevokeAll → las 3 quedan revoked.
- [ ] **T-unit-5**: `TestJWT_SignVerify**` — round-trip funciona.
- [ ] **T-unit-6**: `TestJWT_ExpiredFails**` — JWT con `exp` en el
  pasado → Verify retorna error.
- [ ] **T-e2e-1**: `TestAuthFlow_LoginVerifyMe**` — POST /login →
  POST /verify con OTP (mockeado) → GET /me con cookie → 200
  con user info.
- [ ] **T-e2e-2**: `TestAuthFlow_Refresh**` — login + verify →
  access expira (avanzar reloj en test) → POST /refresh → nuevo
  access válido.
- [ ] **T-e2e-3**: `TestAuthFlow_LogoutRevokes**` — login + verify
  → POST /logout → GET /me con cookie vieja → 401.
- [ ] **T-e2e-4**: `TestAuthDual_BearerStillWorks**` — un MCP
  client con `Authorization: Bearer sk_xxx` sigue funcionando en
  /api/v1/observations (no rompe el flow API key).
- [ ] **T-e2e-5**: `TestCSRF_BlocksMutationWithoutToken**` — POST
  /auth/logout SIN `X-CSRF-Token` → 403.
- [ ] **T-sabotaje**: Comentar el constraint `expires_at > created_at`
  en la migration (sabotaje) + comment-out el filtro `expires_at >
  NOW()` en el middleware → test e2e que crea sesión con
  `expires_at = NULL` y assserta que `/me` retorna 401 (sesión
  expirada) DEBE FALLAR → restaurar constraint + filtro → test
  verde. Documentar en commit body.
