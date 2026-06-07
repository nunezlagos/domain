# HU-13.2-http-auth-middleware

**Origen:** `REQ-13-http-api`
**Persona:** security-officer
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** administrador de la Plataforma Domain
**Quiero** que todas las rutas API estén protegidas por autenticación y autorización
**Para** garantizar que solo clientes autorizados accedan a los recursos según su rol y permisos

## Criterios de aceptación

```gherkin
Feature: HTTP Auth Middleware

  Background:
    Given el servidor API corre en /api/v1/
    And existe un API key válida con permisos de lectura

  Scenario: Acceder sin token retorna 401
    When envío GET /api/v1/observations sin Authorization header
    Then recibo 401 Unauthorized
    And el body contiene error con código "unauthorized"

  Scenario: Token inválido retorna 401
    When envío GET /api/v1/observations con Authorization: Bearer token-invalido
    Then recibo 401 Unauthorized
    And el mensaje indica "invalid or expired API key"

  Scenario: Token expirado retorna 401
    Given existe una API key expirada
    When envío GET /api/v1/observations con esa API key
    Then recibo 401 Unauthorized
    And el mensaje indica "API key expired"

  Scenario: Token válido con permisos insuficientes retorna 403
    Given existe una API key con permiso de solo lectura
    When envío POST /api/v1/observations con esa API key
    Then recibo 403 Forbidden
    And el body contiene error con código "forbidden"

  Scenario: Token válido con permisos correctos accede al recurso
    Given existe una API key con permiso de escritura
    When envío POST /api/v1/observations con esa API key
    Then recibo 201 Created

  Scenario: RBAC por entidad y acción
    Given existe una API key con permiso "observations:read"
    When envío GET /api/v1/observations
    Then recibo 200 OK
    But envío DELETE /api/v1/observations/abc con la misma key
    Then recibo 403 Forbidden

  Scenario: Rate limiting básico
    Given el rate limit es 100 requests/minuto
    When envío 101 requests en menos de 1 minuto
    Then el request 101 retorna 429 Too Many Requests
    And el header Retry-After está presente

  Scenario: Rate limit por API key
    Given dos API keys distintas
    When key A hace 100 requests
    And key B hace 100 requests
    Then ambas llegan a 100 sin ser rate-limited (límite individual)

  Scenario: Request logging registra cada请求
    When envío un request autenticado
    Then se registra: método, path, status, duration, api_key_id, ip
    And el log incluye timestamp ISO8601

  Scenario: Health endpoint no requiere auth
    When envío GET /api/v1/health sin token
    Then recibo 200 OK

  Scenario: CORS headers presentes
    When envío OPTIONS /api/v1/observations con Origin: http://example.com
    Then recibo 200 con headers CORS permitidos

  Scenario: API key desactivada retorna 401
    Given existe una API key que fue desactivada
    When envío GET /api/v1/observations con esa key
    Then recibo 401 Unauthorized
    And el mensaje indica "API key deactivated"
```

## Análisis breve

- **Qué pide realmente:** Middleware chain: Token extraction → API key lookup & validation → Permission check → Rate limit check → Request logging
- **Módulos sospechados:** `internal/api/middleware/auth.go`, `internal/api/middleware/ratelimit.go`, `internal/api/middleware/logger.go`
- **Riesgos / dependencias:** Depende de tabla api_keys (REQ-01), RBAC roles/permissions (REQ-02). Performance overhead en cada request.
- **Esfuerzo tentativo:** L

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
