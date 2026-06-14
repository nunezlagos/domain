# issue-32.1-session-web-auth-cookies-jwt

**Origen:** `REQ-32-dashboard-readiness`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** usuario del dashboard web de domain
**Quiero** loguearme con mi email + OTP (no API key) y mantener sesión activa via cookies httpOnly o JWT
**Para** tener una UX web estándar sin tener que pegar API keys ni tokens en el browser

## Criterios de aceptación

### Escenario 1: Login flow completo

```gherkin
Dado que el user navega a `https://app.tudominio.com/login`
Cuando ingresa su email y hace submit
Entonces el server envía un OTP al email (vía flujo OTP existente, issue-02.5)
Y el user ingresa el código en `https://app.tudominio.com/verify`
Y el server valida el OTP y emite una sesión:
  - Opción A: cookie httpOnly `session_id=<uuid>` (Secure, SameSite=Lax)
  - Opción B: response con `access_token` (JWT, 1h) + `refresh_token` (opaque, 7d)
Y redirige al dashboard
Y el dashboard puede hacer `GET /api/v1/auth/me` y recibir el user info
```

### Escenario 2: `/auth/me` con sesión válida

```gherkin
Dado que tengo cookie `session_id` válida
Cuando hago `GET /api/v1/auth/me`
Entonces el server retorna 200 con `{user_id, org_id, email, role, expires_at}`
Y la respuesta NO incluye el token (no leak)
```

### Escenario 3: `/auth/me` sin sesión

```gherkin
Dado que NO tengo cookie ni token
Cuando hago `GET /api/v1/auth/me`
Entonces el server retorna 401 con `{error_code: "unauthenticated"}`
Y el dashboard redirige a /login
```

### Escenario 4: Sesión expira → refresh funciona

```gherkin
Dado que mi access_token JWT expiró (1h)
Y mi refresh_token sigue válido (7d)
Cuando hago `POST /api/v1/auth/refresh` con el refresh_token
Entonces el server emite un nuevo access_token
Y la próxima request a `/api/v1/auth/me` con el nuevo access_token funciona
Y si el refresh_token también expiró → 401 con "session_expired" (user debe re-login)
```

### Escenario 5: Logout revoca sesión

```gherkin
Dado que tengo sesión activa
Cuando hago `POST /api/v1/auth/logout`
Entonces el server:
  - Invalida el refresh_token (marca revoked en DB)
  - Limpia la cookie (Set-Cookie con MaxAge=0)
Y el response es 204 No Content
Y requests subsecuentes con el token viejo → 401
```

### Escenario 6: Coexistencia con Bearer API key

```gherkin
Dado que un MCP client envía `Authorization: Bearer sk_xxx`
Cuando hace `GET /api/v1/observations`
Entonces el server trata como API key (flujo actual, issue-02.5)
Y `domain-mcp` sigue funcionando sin cambios
Y la nueva sesión web es un paralelo, no un reemplazo
```

### Escenario 7: Sabotaje — sesión sin expiración

```gherkin
Dado que el código de sesión tiene un bug (sabotaje) que pone `expires_at = NULL`
Cuando el user hace logout
Entonces la sesión sigue activa (bug: el filtro `expires_at > NOW()` retorna la sesión zombie)
Y el test e2e que assserta "sesión sin expires_at es rechazada" DEBE FALLAR
Cuando restauro la lógica de expiración obligatoria
Entonces el test verde
```

### Escenario 8: Edge case — múltiples sesiones del mismo user

```gherkin
Dado que el user tiene sesión activa en Chrome
Y hace login también en Firefox
Entonces ambas sesiones son independientes (cada una con su cookie + refresh_token)
Y logout en Chrome NO afecta Firefox
Y logout global (logout_all) revoca todas
```

## Notas

- El flujo OTP YA EXISTE (`internal/auth/otp/`). Reusar la mayor
  parte. Lo nuevo es la capa de sesión.
- Decisión de storage: cookies httpOnly (más seguro, no expone JWT
  en localStorage) vs JWT en response (más flexible, el cliente
  decide dónde guardarlo). Default: cookies httpOnly. JWT es
  opt-in via header `X-Auth-Response: json`.
- Las sesiones se persisten en tabla nueva `web_sessions` (no
  `api_keys` que es para service-to-service).
- Refresh tokens son OPACOS (no JWTs), stored en DB con hash.
  Esto permite revoke inmediato (borrar la fila).
