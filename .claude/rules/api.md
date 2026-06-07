# HTTP API Conventions — Domain

Convenciones para `/api/v1/*` endpoints. Enforcement automatizado en HU-13.9 response-shape-linter.

## URL conventions

- **Versioning**: `/api/v1/`, `/api/v2/` (URL versioning, ver HU-13.8)
- **Resources**: plural, kebab-case si multi-word (`/agent-runs`, no `/agent_runs` ni `/agentRuns`)
- **Sub-resources**: `/orgs/:org_id/projects` para hierarchical
- **Actions no-CRUD**: `POST /agents/:id/run`, `POST /runs/:id/cancel` (verbo en URL, no método HTTP)
- **Batch**: sufijo `/batch` (`POST /observations/batch` — HU-13.5)
- **Search**: `GET /search?q=...` global (HU-03.7)
- **Identifiers**: UUIDs en path (`/runs/:id`); slugs cuando user-meaningful (`/projects/:slug`)

## HTTP methods

| operación | método | status éxito |
|-----------|--------|--------------|
| List | GET | 200 |
| Get single | GET | 200 |
| Create | POST | 201 + Location header |
| Update parcial | PATCH | 200 |
| Replace | PUT | 200 (no usar salvo CAS necesario) |
| Delete | DELETE | 204 sin body |
| Custom action | POST | 200 o 202 (async) |
| Batch | POST `/batch` | 207 Multi-Status |

## Status codes

| código | significado |
|--------|-------------|
| 200 | OK |
| 201 | Created (Location header obligatorio) |
| 202 | Accepted (async; devolver job_id) |
| 204 | No Content (delete) |
| 207 | Multi-Status (batch) |
| 304 | Not Modified (ETag match) |
| 400 | Bad request (malformed JSON, type error) |
| 401 | No autenticado |
| 403 | Autenticado pero sin permiso |
| 404 | No existe O no accesible (anti-enumeration) |
| 409 | Conflicto (idempotency mismatch, lock conflict) |
| 410 | Gone (resource removed, version sunset) |
| 412 | Precondition Failed (If-Match optimistic concurrency) |
| 413 | Payload too large (batch >5000, body >N MB) |
| 422 | Validation failed (campos requeridos, formato) |
| 429 | Rate limited (Retry-After header) |
| 500 | Internal error (logear, NO leak stack) |
| 503 | Service Unavailable (degraded, depends) |

## Error response shape (RFC 7807-inspired)

```json
{
  "error": {
    "code": "validation_failed",
    "message": "Human readable summary",
    "details": [
      {"field": "email", "code": "invalid_format", "message": "..."},
      {"field": "rut",   "code": "invalid_check_digit", "message": "..."}
    ],
    "request_id": "<uuid>",
    "trace_id": "<otel trace_id>",
    "docs_url": "https://docs.domain.sh/errors/validation_failed"
  }
}
```

Reglas:
- `code` es máquina-leíble snake_case, estable across versions
- `message` puede cambiar, es human-facing
- `details` array para field-level errors (en 422)
- `request_id` SIEMPRE presente
- `trace_id` cuando hay tracing activo (REQ-17)
- `docs_url` cuando hay doc específica del error

## Success response shape

### Single resource

```json
{
  "data": { ...resource... }
}
```

### List

```json
{
  "data": [...],
  "pagination": {
    "next_cursor": "<base64url>",
    "has_more": true,
    "limit": 50
  }
}
```

NUNCA devolver array root-level: siempre envolver en `data`.

### Action

```json
{
  "data": { ...result... },
  "meta": { "duration_ms": 234, "warnings": [...] }
}
```

## Headers obligatorios

### Request
- `Authorization: Bearer <api_key>` para autenticación
- `Content-Type: application/json` para POST/PATCH/PUT
- `Idempotency-Key: <uuid>` recomendado en POST/PATCH/DELETE (HU-13.4)
- `If-Match: "<etag>"` opcional para optimistic concurrency
- `X-Organization-Id: <uuid>` opcional si user es member de N orgs

### Response (siempre)
- `Content-Type: application/json; charset=utf-8`
- `X-Request-Id: <uuid>` (correlación con logs)
- `X-Rate-Limit-Limit`, `X-Rate-Limit-Remaining`, `X-Rate-Limit-Reset` cuando aplica

### Response condicional
- `ETag: "<hash>"` en GET single (HU-13.7)
- `Last-Modified` en GET con resource modificable
- `Cache-Control` según endpoint policy
- `Location` en 201 Created
- `Retry-After` en 429 y 503
- `Deprecation` / `Sunset` para versions deprecated (HU-13.8)
- `Idempotent-Replayed: true` cuando se devuelve cached (HU-13.4)

## Query params

- `limit` (default 50, max 200)
- `cursor` (opaque base64url) — preferido sobre offset
- `sort` formato `field:asc|desc` (default específico por endpoint)
- `filter[field]=value` o `field=value` simple — documentar por endpoint
- Booleanos: `true`/`false` strings (no `1`/`0`)
- Multi-value: csv `tags=a,b,c` o repetir `tags=a&tags=b`
- Timezone: ISO 8601 con offset (`2026-06-07T12:00:00-04:00`)

## Datos sensibles

- NUNCA echar API keys, passwords, OTP codes, payment data en response (ni siquiera redacted)
- Para tokens nuevos creados (HU-02.7 verify-otp): devolver UNA vez en `data.api_key`, después solo el `key_prefix`
- PII en responses: solo si el caller tiene RBAC para ver

## Async operations

```
POST /flows/:id/run
→ 202 Accepted
{ "data": { "run_id": "...", "status": "pending" } }
Location: /api/v1/runs/<id>

GET /runs/:id → estado actual
GET /runs/:id/stream → SSE con eventos
```

## Anti-patterns prohibidos

- ❌ Array root-level en response (siempre `{data: [...]}`)
- ❌ `200 OK` con `{"success": false}` body — usar status code correcto
- ❌ `404` que distingue "no existe" vs "no autorizado" (enumeration leak)
- ❌ Stack traces en 500 (logear sí, response no)
- ❌ Verbos en URL: `/getUser` ❌ → `GET /users/:id` ✓
- ❌ snake_case en URLs (usar kebab-case)
- ❌ Status code 200 en error
- ❌ Vary semantics across versions sin bump major
