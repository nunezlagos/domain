# Design: issue-32.1-session-web-auth-cookies-jwt

## Contexto

El dashboard web (proyecto futuro) requiere autenticación de usuario
final, no service-to-service. El patrón estándar es:
1. Login con email + OTP (el user prueba que controla el email).
2. El server emite una sesión persistible (cookie httpOnly o JWT).
3. El cliente envía la cookie/token en cada request.
4. Refresh tokens permiten extender la sesión sin pedir password.

Hoy domain solo tiene API key Bearer (issue-02.5). La capa de
sesión web es ADITIVA: no rompe el flujo API key, que sigue
funcionando para CLIs y MCP.

## Decisión arquitectónica

**Estrategia:** reusar OTP existente + nueva tabla `web_sessions` +
middleware de auth dual (Bearer OR Session).

1. **Migración nueva** (`migrations/000091_web_sessions.sql`):
   ```sql
   CREATE TABLE web_sessions (
     id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
     user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
     organization_id UUID NOT NULL REFERENCES organizations(id),
     refresh_token_hash VARCHAR(64) NOT NULL UNIQUE,  -- sha256 hex
     user_agent TEXT,
     ip INET,
     created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
     last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
     expires_at TIMESTAMPTZ NOT NULL,  -- refresh expiry
     revoked_at TIMESTAMPTZ
   );
   CREATE INDEX idx_web_sessions_user ON web_sessions(user_id) WHERE revoked_at IS NULL;
   CREATE INDEX idx_web_sessions_refresh ON web_sessions(refresh_token_hash) WHERE revoked_at IS NULL;
   ```

2. **Endpoints nuevos** (handler en `internal/api/handler/auth_web.go`):
   - `POST /api/v1/auth/login` — body `{email}` → 202 con
     "OTP sent". Internamente invoca el `otp.RequestOTP` existente.
   - `POST /api/v1/auth/verify` — body `{email, otp_code}` →
     crea sesión, retorna Set-Cookie + body con `expires_at` (o
     JSON `{access_token, refresh_token}` si se pidió
     `X-Auth-Response: json`).
   - `POST /api/v1/auth/refresh` — body o header con
     `refresh_token` → emite nuevo access token (cookie o JSON).
   - `POST /api/v1/auth/logout` — revoke la sesión actual, limpia
     cookie. 204.
   - `POST /api/v1/auth/logout-all` — revoke todas las sesiones
     del user. 204.
   - `GET /api/v1/auth/me` — retorna info del user actual.

3. **Access token (JWT):** payload
   `{sub: user_id, org_id, session_id, exp, iat}`, firmado con
   `DOMAIN_MASTER_KEY` (HMAC-SHA256). TTL 1h.

4. **Refresh token:** opaque random 32 bytes, base64url.
   Server solo guarda el hash. TTL 7d (configurable).

5. **Middleware de auth dual:** reemplazar
   `apikey.Middleware` por una composición:
   ```go
   func AuthDualMW(apikeyMW, sessionMW) http.Handler {
     return http.HandlerFunc(func(w, r) {
       // 1. Si Authorization: Bearer sk_xxx → apikeyMW
       if isAPIKey(r) { apikeyMW.ServeHTTP(w, r); return }
       // 2. Si Cookie session_id OR Authorization: Bearer <jwt> → sessionMW
       if isSession(r) { sessionMW.ServeHTTP(w, r); return }
       // 3. Allowlist (login, verify, health, etc) pasa
       // 4. Else 401
     })
   }
   ```

6. **Config:**
   - `DOMAIN_SESSION_TTL_HOURS=168` (7d).
   - `DOMAIN_ACCESS_TOKEN_TTL_MINUTES=60`.
   - `DOMAIN_COOKIE_SECURE=true` (default true en prod).
   - `DOMAIN_COOKIE_DOMAIN=.tudominio.com` (para compartir entre
     subdominios).

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Sesión server-side solo (sin JWT) | Funciona pero el cliente tiene que enviar cookie siempre. JWT permite stateless verification. Mezcla es mejor. |
| B | Solo JWT (sin refresh token) | Refresh tokens permiten revoke sin esperar al expiry. Sin ellos, un token robado vive 1h. |
| C | Reusar tabla `api_keys` con un tipo "session" | Mezcla dominios (service-to-service vs user). Tabla dedicada es más clara. |
| D | OAuth2 / OIDC (login con Google) | Out of scope. El user pidió email+OTP. OAuth puede venir después como SSO opcional. |

## Por qué tabla dedicada + auth dual gana

- **No invasivo:** el flow API key sigue funcionando. Solo
  agregamos un path paralelo.
- **Revocable:** refresh tokens en DB con `revoked_at` permiten
  kill switch inmediato (e.g. "sesiones en X país están
  comprometidas").
- **Estandard:** cookies httpOnly + CSRF token es el patrón
  OWASP. JWT en response es opt-in para clients que lo prefieren.
- **Auditable:** cada sesión tiene `user_agent`, `ip`,
  `last_used_at`. Se puede listar en `/auth/sessions` (futuro).

## Detalle de implementación

- Migración: `migrations/000091_web_sessions.sql`.
- Service: `internal/auth/websession/service.go` con
  `Create(userID, orgID, userAgent, ip) (access, refresh, error)`,
  `Refresh(refreshToken) (newAccess, newRefresh, error)`,
  `Revoke(sessionID) error`, `RevokeAll(userID) error`,
  `GetByID(sessionID) (*Session, error)`.
- Handler: `internal/api/handler/auth_web.go` con los 6
  endpoints.
- Middleware: `internal/auth/websession/middleware.go` con
  `Middleware` que extrae session_id de cookie O JWT de
  Authorization header.
- JWT: `internal/auth/jwt/jwt.go` con `Sign(claims)` y
  `Verify(token)`. Usa `golang-jwt/jwt/v5`.
- CSRF: middleware `internal/api/middleware/csrf.go` que para
  métodos no-GET/HEAD/OPTIONS requiere header `X-CSRF-Token`
  igual al valor de la cookie `csrf_token` (double-submit
  pattern).

## Riesgos

- **R1:** CSRF si solo cookie sin CSRF token. **Mitigación:**
  doble submit pattern con cookie `csrf_token` (no httpOnly) +
  header `X-CSRF-Token` en mutaciones.
- **R2:** Refresh token en DB crece. **Mitigación:** job nightly
  purga `revoked_at IS NOT NULL AND revoked_at < NOW() - 30 days`
  Y `expires_at < NOW() - 30 days`. Configurable.
- **R3:** Si el `DOMAIN_MASTER_KEY` se rota, todos los JWTs
  emitidos antes quedan inválidos. **Mitigación:** keyring
  multi-versión (issue-02.3) ya está implementado; reusar.
