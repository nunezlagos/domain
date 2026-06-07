# Design: HU-13.5-bulk-batch-endpoints

## Decisión arquitectónica

**Response:** HTTP 207 Multi-Status estándar.
**all_or_nothing:** `pgx.CopyFrom` para bulk insert atómico.
**best_effort:** per-item transactions; aggregate errors.
**Limit:** 5000 items por batch (configurable `DOMAIN_BATCH_MAX_ITEMS`).

## Response shape

```json
{
  "results": [
    {"index": 0, "status": 201, "data": {"id":"..."}},
    {"index": 1, "status": 422, "error": {"code":"validation_failed","details":[...]}},
    {"index": 2, "status": 201, "data": {"id":"..."}}
  ],
  "summary": {"total": 3, "succeeded": 2, "failed": 1}
}
```

## TDD plan

1. 500 items happy
2. all_or_nothing item 250 fail → rollback
3. best_effort partial
4. 5001 → 413
5. Idempotency replay
6. Bulk delete permisos mixtos
