# Tasks: issue-32.2-cors-allowlist-configurable

## Backend

- [ ] **T1**: Agregar `github.com/rs/cors` a `go.mod`.

- [ ] **T2**: Crear `internal/api/middleware/cors.go`:
  - `NewCORS(origins []string) *cors.Cors` con la lógica de
    design.md (allowlist, wildcard, default deny).
  - `OriginsFromEnv(envValue string) []string` helper: CSV split +
    trim spaces + filter vacíos.

- [ ] **T3**: Wire en `cmd/domain/main.go`:
  - Leer `DOMAIN_CORS_ORIGINS` con `OriginsFromEnv`.
  - Si vacío: no wrappear con CORS (skip).
  - Si tiene: wrappear el sub-mux `/api/` con
    `corsHandler.Handler(apiSubMux)`.
  - Loggear al boot cuántos origins están configurados.

- [ ] **T4**: Logging de denials: wrapper middleware
  `CORSDenyLogger` que intercepta responses sin
  `Access-Control-Allow-Origin` cuando el request tenía `Origin`
  header, y loggea con `slog.Warn("CORS denied", "origin",
  origin, "method", method, "path", path)`.

- [ ] **T5**: Config: agregar `CORSOrigins []string` a
  `config.Config` (parseado de la env var).

- [ ] **T6**: Documentar en `.env.example` la variable con un
  ejemplo comentado.

## Tests

- [ ] **T-unit-1**: `TestOriginsFromEnv_Empty**` — `""` → `[]`.
- [ ] **T-unit-2**: `TestOriginsFromEnv_Multiple**` —
  `"a.com,b.com"` → `["a.com", "b.com"]`.
- [ ] **T-unit-3**: `TestOriginsFromEnv_TrimsSpaces**` —
  `" a.com , b.com "` → `["a.com", "b.com"]`.
- [ ] **T-unit-4**: `TestNewCORS_EmptyOrigins**` — `[]` → middleware
  que NO agrega CORS headers (default deny).
- [ ] **T-unit-5**: `TestNewCORS_Wildcard**` — `["*"]` → middleware
  con `Allow-Credentials: false` y loggea WARNING (capturar con
  slog handler de test).
- [ ] **T-unit-6**: `TestNewCORS_MultipleOrigins**` —
  `["a.com", "b.com"]` → middleware que valida contra el set.
- [ ] **T-e2e-1**: `TestCORS_AllowedOrigin**` — preflight desde
  `https://app.tudominio.com` con `Origin` header → response
  tiene `Access-Control-Allow-Origin: https://app.tudominio.com`
  + `Access-Control-Allow-Credentials: true` + Vary header.
- [ ] **T-e2e-2**: `TestCORS_NotAllowedOrigin**` — preflight desde
  `https://evil.com` → response NO tiene
  `Access-Control-Allow-Origin` matching. El log tiene WARNING
  "CORS denied".
- [ ] **T-e2e-3**: `TestCORS_PreflightAllowed**` — OPTIONS con
  `Access-Control-Request-Method: POST` → response con
  `Access-Control-Allow-Methods: GET,POST,...` y
  `Access-Control-Max-Age: 86400`.
- [ ] **T-e2e-4**: `TestCORS_NoOriginServerToServer**` — request SIN
  `Origin` header (e.g. `Authorization: Bearer sk_xxx` desde
  domain-mcp) → response OK sin CORS headers. CORS no rompe el
  flow API key.
- [ ] **T-e2e-5**: `TestCORS_PortMismatch**` — origin
  `https://app.tudominio.com:3000` cuando allowlist tiene
  `https://app.tudominio.com` → response SIN CORS allow + log de
  mismatch.
- [ ] **T-sabotaje**: Comentar el filter check
  `if containsOrigin(allowedOrigins, origin)` en
  `NewCORS` (sabotaje: siempre acepta) → test e2e-2 DEBE FALLAR
  (el response tiene `Access-Control-Allow-Origin: *` para
  evil.com) → restaurar check → test verde. Documentar sabotaje
  en commit body.
