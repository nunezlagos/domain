# issue-13.7-http-caching-etags

**Origen:** `REQ-13-http-api`
**Prioridad tentativa:** baja
**Tipo:** feature

## Historia de usuario

**Como** cliente API (SDK, browser, CDN)
**Quiero** soporte ETag / If-None-Match / Last-Modified / If-Modified-Since
**Para** evitar transferir bodies completos cuando nada cambió

## Criterios de aceptación

### Escenario 1: GET con ETag

```gherkin
Dado que GET /api/v1/observations/:id
Cuando el server responde 200
Entonces incluye `ETag: "<sha256-short>"` y `Last-Modified: <RFC1123>`
```

### Escenario 2: If-None-Match hit

```gherkin
Dado que el cliente envía `If-None-Match: "abc123"`
Y el server computa ETag actual = "abc123"
Entonces responde 304 Not Modified sin body
```

### Escenario 3: If-Modified-Since hit

```gherkin
Dado que el cliente envía `If-Modified-Since: Wed, 07 Jun 2026 12:00:00 GMT`
Y `updated_at <= esa fecha`
Entonces 304
```

### Escenario 4: Cache-Control headers por endpoint

```gherkin
Dado que GET /api/v1/projects/:id (entidad estable)
Entonces incluye `Cache-Control: private, max-age=60`
Y GET /api/v1/runs/:id (in-progress)
Entonces incluye `Cache-Control: private, no-store`
```

### Escenario 5: Compute ETag eficiente

```gherkin
Dado que entity tiene `updated_at` y `id`
Entonces ETag = first 16 chars de sha256(updated_at_unix + ":" + id)
Y NO se serializa el body completo solo para hashearlo
```

### Escenario 6: PATCH con If-Match optimistic concurrency

```gherkin
Dado que PATCH /api/v1/observations/:id con `If-Match: "abc123"`
Y el ETag actual es "def456" (alguien más modificó)
Entonces 412 Precondition Failed
```

## Análisis breve

- **Qué pide:** ETag/Last-Modified/Cache-Control + If-None-Match + If-Match optimistic
- **Esfuerzo:** S
- **Riesgos:** ETag stale si updated_at no se actualiza siempre; cache breaks PII redaction (cliente cache datos sensibles)
