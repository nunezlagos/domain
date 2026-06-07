# Design: HU-13.7-http-caching-etags

## ETag scheme

```
etag = first 16 chars of hex(sha256(updated_at_unix_ns + ":" + id))
```

Weak ETag `W/` opcional para colecciones (paginación).

## Cache-Control policy

| endpoint pattern | Cache-Control |
|------------------|---------------|
| GET /projects/:id | private, max-age=60 |
| GET /agents/:id | private, max-age=300 |
| GET /runs/:id (active) | private, no-store |
| GET /runs/:id (terminal) | private, max-age=3600 |
| GET /observations/:id | private, max-age=60 |
| GET /me | private, no-store |
| auth endpoints | no-store |

## Optimistic concurrency

```
PATCH /api/v1/observations/:id
If-Match: "abc123"
→ if current_etag != "abc123": 412 Precondition Failed
→ else: apply patch + return 200 with new ETag
```

## TDD plan

1. ETag presente GET
2. 304 If-None-Match match
3. PATCH If-Match mismatch 412
4. Cache-Control por endpoint
5. ETag stable across requests
6. Update changes ETag
