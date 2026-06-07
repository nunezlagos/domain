# HU-13.6-cursor-pagination

**Origen:** `REQ-13-http-api`
**Persona:** integrator
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** cliente API que itera datasets grandes
**Quiero** cursor-based pagination en lugar de offset
**Para** no romper con datasets >100k filas ni perder/duplicar items en inserts concurrentes

## Criterios de aceptación

### Escenario 1: Cursor opaque

```gherkin
Dado que GET /api/v1/observations?limit=50
Cuando el server responde
Entonces devuelve `{"data":[...], "pagination":{"next_cursor":"<opaque>", "has_more":true}}`
Y next_cursor es base64url(json{last_id, last_created_at, filters_hash, version})
```

### Escenario 2: Next page

```gherkin
Dado que recibí next_cursor en página 1
Cuando GET /api/v1/observations?cursor=<opaque>&limit=50
Entonces se decodifica cursor
Y se valida filters_hash match con request actual
Y se devuelven 50 siguientes (sorted by created_at DESC, id DESC)
Y next_cursor nuevo o has_more=false
```

### Escenario 3: Cursor tampered

```gherkin
Dado que el cliente modifica cursor
Cuando el server decodifica
Entonces 400 "invalid cursor"
```

### Escenario 4: Filters mismatch

```gherkin
Dado que el cursor fue generado con filter project_id=X
Cuando reuso ese cursor pero con project_id=Y
Entonces 400 "cursor filters mismatch"
```

### Escenario 5: Sort options

```gherkin
Dado que GET ?sort=created_at:asc o ?sort=created_at:desc (default)
Cuando se paginan
Entonces cursor respeta sort order
Y same cursor con sort distinto → 400
```

### Escenario 6: Coexistencia con offset legacy

```gherkin
Dado que el cliente envía ?offset=200&limit=50 (legacy)
Entonces se devuelve `Deprecation` header
Y funciona pero capado a offset≤10000 (más → error)
```

## Análisis breve

- **Qué pide:** cursor opaque + sort stable + filter hash + legacy offset deprecated
- **Esfuerzo:** S
- **Riesgos:** cursor stale tras schema migration; deep paginate aún costoso si filter no indexed
