# issue-32.2-cors-allowlist-configurable

**Origen:** `REQ-32-dashboard-readiness`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** developer del dashboard web (hospedado en `app.tudominio.com`)
**Quiero** que el server domain acepte requests cross-origin desde mi dominio
**Para** que la SPA pueda llamar al API sin que el browser bloquee por CORS

## Criterios de aceptación

### Escenario 1: CORS habilitado para origins en allowlist

```gherkin
Dado que `DOMAIN_CORS_ORIGINS=https://app.tudominio.com,https://dashboard.tudominio.com` está seteada
Cuando el dashboard hace `fetch('https://api.tudominio.com/api/v1/auth/me', {credentials: 'include'})`
Entonces el response incluye header `Access-Control-Allow-Origin: https://app.tudominio.com`
Y `Access-Control-Allow-Credentials: true`
Y el browser NO bloquea la request
```

### Escenario 2: Origin no en allowlist → no CORS headers

```gherkin
Dado que `DOMAIN_CORS_ORIGINS=https://app.tudominio.com`
Cuando `https://evil.com` intenta hacer request al API
Entonces el response NO incluye `Access-Control-Allow-Origin: https://evil.com`
Y el browser bloquea la request (CORS error)
Y el server loggea: "CORS denied origin: https://evil.com"
```

### Escenario 3: Preflight OPTIONS pasa

```gherkin
Dado que `https://app.tudominio.com` está en allowlist
Cuando el browser hace preflight `OPTIONS /api/v1/observations` con:
  - Origin: https://app.tudominio.com
  - Access-Control-Request-Method: POST
  - Access-Control-Request-Headers: Authorization, Content-Type
Entonces el response 200 (o 204) incluye:
  - Access-Control-Allow-Origin: https://app.tudominio.com
  - Access-Control-Allow-Methods: GET, POST, PATCH, DELETE, OPTIONS
  - Access-Control-Allow-Headers: Authorization, Content-Type, X-CSRF-Token
  - Access-Control-Allow-Credentials: true
  - Access-Control-Max-Age: 86400
Y el método real posterior funciona
```

### Escenario 4: Default deny — sin env var

```gherkin
Dado que `DOMAIN_CORS_ORIGINS` NO está seteada
Cuando el dashboard intenta CORS request
Entonces el server NO agrega headers CORS
Y el browser bloquea
Y en el log: "CORS not configured; set DOMAIN_CORS_ORIGINS to enable"
```

### Escenario 5: Wildcard support opcional

```gherkin
Dado que `DOMAIN_CORS_ORIGINS=*` (wildcard, solo válido en dev)
Cuando el dashboard hace CORS request
Entonces el server agrega `Access-Control-Allow-Origin: *` (sin credentials)
Y loggea WARNING: "CORS wildcard enabled; NOT for production"
Y `Access-Control-Allow-Credentials` se omite (los browsers no permiten * + credentials)
```

### Escenario 6: Múltiples origins con Vary: Origin

```gherkin
Dado que `DOMAIN_CORS_ORIGINS=https://app.tudominio.com,https://staging.tudominio.com`
Cuando un cliente de `app` hace request Y otro de `staging` hace request
Entonces el response del server incluye `Vary: Origin` (para que el cache no mezcle)
Y cada cliente ve su propio `Access-Control-Allow-Origin`
```

### Escenario 7: Sabotaje — CORS abierto a todos los origins

```gherkin
Dado que el código de CORS tiene un bug (sabotaje) que siempre retorna
`Access-Control-Allow-Origin: *` independientemente de la env var
Cuando un origin no en allowlist hace request
Entonces el response tiene `Access-Control-Allow-Origin: *` (incorrecto)
Y el test e2e que assserta "origin evil.com NO en CORS headers" DEBE FALLAR
Cuando restauro el filter check
Entonces el test verde
```

### Escenario 8: Edge case — origin con puerto

```gherkin
Dado que `DOMAIN_CORS_ORIGINS=https://app.tudominio.com`
Y el dashboard está en `https://app.tudominio.com:3000` (dev con puerto)
Cuando hace request
Entonces el server NO matchea (puerto diferente) → CORS denied
Y loggea: "CORS origin mismatch: got app.tudominio.com:3000, expected app.tudominio.com"
```

## Notas

- CORS se aplica SOLO a `/api/v1/*` (no a `/health` ni
  `/api/v1/openapi.json` que son públicos).
- CORS es independiente del middleware de auth: la request puede
  tener CORS OK pero auth fail (origen válido + token inválido =
  401 con CORS headers, no 403 sin CORS).
- Implementación: NO usar un middleware genérico que parchea
  headers. Usar `github.com/rs/cors` (librería estándar) o
  similar, configurada explícitamente.
