# Design: issue-13.6-cursor-pagination

## Cursor shape

```json
{"v":1,"id":"<uuid>","ts":"2026-06-07T12:00:00Z","sort":"created_at:desc","h":"<sha256>"}
```

base64url encoded.

## Query template

```sql
-- desc default
SELECT * FROM observations
WHERE org_id = $1
  AND (created_at, id) < ($cursor_ts, $cursor_id)
  AND <filters>
ORDER BY created_at DESC, id DESC
LIMIT $limit + 1;   -- +1 para detectar has_more

-- asc
SELECT * FROM observations
WHERE org_id = $1
  AND (created_at, id) > ($cursor_ts, $cursor_id)
ORDER BY created_at ASC, id ASC
LIMIT $limit + 1;
```

## Response

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

## TDD plan

1. First page con limit 50 + has_more=true
2. Next page con cursor → 50 más, no overlap
3. Tampered → 400
4. Filters_hash mismatch → 400
5. Sort asc/desc
6. Legacy offset Deprecation header
7. Performance 100k filas p99 <100ms
