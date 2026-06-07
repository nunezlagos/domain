# Proposal: HU-13.7-http-caching-etags

## Intención

Implementar HTTP caching estándar con ETag (sha256 abreviado), Last-Modified, Cache-Control per-endpoint, If-None-Match (304), If-Match (412 optimistic concurrency).

## Scope

**Incluye:**
- Middleware que computa ETag para GET single-resource
- Headers Last-Modified, Cache-Control
- If-None-Match → 304
- If-Match en PATCH/DELETE → 412 si mismatch
- Cache-Control policies por endpoint type (stable vs volatile)

**No incluye:**
- Server-side caching layer (Redis/Memcached)
- Public CDN caching (private cache only por defecto)

## Enfoque técnico

1. ETag = first 16 chars sha256(updated_at_unix:id)
2. Helper en handler GET: setea ETag + Last-Modified
3. PATCH/DELETE: validar If-Match si presente
4. Cache-Control en route declarativo por type

## Riesgos

- Update sin tocar updated_at: rule "siempre actualizar"
- Sensitive data cached: Cache-Control private + corto max-age o no-store
- 304 body must be empty: respect HTTP spec

## Testing

- ETag presente en GET
- 304 con If-None-Match match
- 304 con If-Modified-Since
- PATCH If-Match mismatch → 412
- Cache-Control varía por endpoint
- ETag stable across same data
