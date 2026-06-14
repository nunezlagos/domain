# Design: issue-09.8-external-signals

## Decisión arquitectónica

**Step type:** `await_signal` (extiende REQ-09 issue-09.2).
**Wake mechanism:** Postgres LISTEN/NOTIFY a workers para wake-up sin polling.
**Persistence:** signals "early" se guardan en ventana de TTL configurable.

## Schema

```sql
CREATE TABLE flow_run_signals_pending (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  flow_run_id UUID NOT NULL REFERENCES flow_runs(id) ON DELETE CASCADE,
  step_id VARCHAR(100) NOT NULL,
  signal_name VARCHAR(100) NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  UNIQUE (flow_run_id, step_id)
);
CREATE INDEX ON flow_run_signals_pending (signal_name);

CREATE TABLE flow_signals_delivered (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  flow_run_id UUID NOT NULL REFERENCES flow_runs(id) ON DELETE CASCADE,
  signal_name VARCHAR(100) NOT NULL,
  payload JSONB,
  delivered_at TIMESTAMPTZ DEFAULT NOW(),
  delivered_by UUID REFERENCES users(id)
);
```

## Endpoints

| método | path | role |
|--------|------|------|
| POST | /api/v1/runs/:id/signals | run.signal |
| POST | /api/v1/flows/:slug/signals | run.signal en org |
| GET | /api/v1/runs/:id/signals/pending | run.read |

## TDD plan

1. await_signal + signal happy
2. Timeout retry
3. Broadcast a N runs
4. Sin pending → 409
5. RBAC enforce
6. Signal a run cancelled → no-op
7. Early signal en window → entregado al llegar al step
