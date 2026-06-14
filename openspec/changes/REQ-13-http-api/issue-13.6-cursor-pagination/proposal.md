# Proposal: issue-13.6-cursor-pagination

## Intención

Cursor-based pagination opaque para todos los list endpoints. Cursor encoded contiene last_id + sort field + filters_hash. Legacy offset deprecated pero funcional con cap.

## Scope

**Incluye:**
- Helper `pagination.Cursor` encode/decode (base64url + JSON)
- Sort stable (id como tiebreaker)
- Filters_hash en cursor para invalidar si change
- Aplicado a todos los list endpoints
- Header `Deprecation` + cap en offset legacy

**No incluye:**
- Keyset pagination multi-column complex (solo created_at + id)
- Bidirectional (prev cursor) en v1 (futuro)

## Enfoque técnico

1. Cursor JSON `{v:1, id, ts, h}`
2. Query: `WHERE (created_at, id) < (?, ?) ORDER BY created_at DESC, id DESC LIMIT ?`
3. Filters_hash = SHA256(canonical filters JSON)
4. Versioned cursor for future migrations

## Riesgos

- Cursor invalido tras schema change: versión + graceful error
- Filter hash conflict edge case: unlikely con SHA256

## Testing

- Cursor opaque
- Next page no overlap
- Tampered → 400
- Filters mismatch → 400
- Sort options
- Legacy offset deprecated
- Performance 100k rows
