# Design: issue-26.6-backpressure-queue

## Queue configs en BD

```sql
CREATE TABLE queue_configs (
  name VARCHAR(50) PRIMARY KEY,
  max_global INT NOT NULL DEFAULT 10000,
  drop_policy VARCHAR(20) NOT NULL DEFAULT 'none',  -- none|fifo
  enabled BOOLEAN DEFAULT true
);
```

## Helper

```go
// internal/backpressure/check.go
func CheckCapacity(ctx, pool, queue string, orgID uuid.UUID, planMax int) error {
  // global
  var depth int
  pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE status = 'pending'", queue)).Scan(&depth)
  if depth >= cfg.MaxGlobal { return ErrQueueFullGlobal }
  // per-org
  var orgDepth int
  pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE status = 'pending' AND organization_id = $1", queue), orgID).Scan(&orgDepth)
  if orgDepth >= planMax { return ErrQueueFullOrg }
  return nil
}
```

## Métricas

```
domain_queue_depth{queue,organization_id}  (org cardinality si <1000 orgs)
domain_queue_shed_total{queue,reason}
domain_queue_dropped_total{queue,reason}
```

## TDD plan

1. Global cap → shed 429
2. Per-org cap → shed 429
3. Métricas depth correctos
4. Drop FIFO en queue marcada
