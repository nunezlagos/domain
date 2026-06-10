# issue-02.5-rate-limit-pii

**Origen:** `REQ-02-auth-security`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** operador de la plataforma
**Quiero** limitar la tasa de requests por API key (token bucket) y redactar automáticamente datos PII (etiquetados como `<private>`)
**Para** proteger el sistema contra abusos y prevenir la exposición accidental de información sensible en logs y almacenamiento

## Criterios de aceptación

### Escenario 1: Rate limiting por API key

```gherkin
Dado que el límite es 100 requests por minuto por API key
Cuando un API key hace 100 requests en menos de 60 segundos
Entonces todos los requests son exitosos (200)

Cuando el mismo API key hace el request 101 dentro del mismo minuto
Entonces recibe `429 Too Many Requests`
Y el header `Retry-After` indica los segundos restantes
Y el header `X-RateLimit-Remaining` es `0`
Y el header `X-RateLimit-Reset` contiene el timestamp Unix de reset
```

### Escenario 2: Token bucket se rellena

```gherkin
Dado que un API key fue rate-limited
Cuando espero el tiempo suficiente (según Retry-After)
Y hago un nuevo request
Entonces recibo `200 OK`
Y `X-RateLimit-Remaining` es `99` (o el máximo menos 1)
```

### Escenario 3: Límites configurables

```gherkin
Dado que la variable `DOMAIN_RATE_LIMIT_REQUESTS` está en `50`
Y `DOMAIN_RATE_LIMIT_WINDOW` está en `30s`
Cuando un API key hace 50 requests en 30 segundos
Entonces el request 51 recibe `429`

Dado que no se configuraron límites explícitos
Cuando un API key hace requests
Entonces se aplica el límite por defecto (100/minuto)
```

### Escenario 4: PII redaction en responses

```gherkin
Dado que un texto contiene `<private>user@email.com</private>`
Cuando el sistema procesa y almacena ese texto
Entonces el texto almacenado contiene `[REDACTED]` en lugar del contenido

Dado que un texto contiene `<private>token-secreto-123</private>`
Cuando el texto se devuelve en un response
Entonces el contenido entre tags es reemplazado por `[REDACTED]`
```

### Escenario 5: PII redaction en logs

```gherkin
Dado que un request body contiene `<private>password123</private>`
Cuando el sistema loguea el body (debug level)
Entonces el log muestra `<private>[REDACTED]</private>`
```

### Escenario 6: Sin tags PII no hay redacción

```gherkin
Dado que un texto no contiene tags `<private>`
Cuando el sistema procesa el texto
Entonces el texto se almacena y devuelve sin modificación
```

## Análisis breve

- **Qué pide realmente:** Rate limiting con token bucket por API key (configurable) + PII redaction automática de contenido entre tags `<private>` en storage y logs.
- **Módulos sospechados:** `internal/api/middleware/ratelimit.go`, `internal/sanitize/`, `internal/api/middleware/pii.go`
- **Riesgos / dependencias:** El rate limit debe ser eficiente en memoria (no por request a DB). La redacción PII debe aplicarse en la capa correcta (antes de storage, antes de log).
- **Esfuerzo tentativo:** M

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
