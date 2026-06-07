# Design: HU-09.9-saga-compensation

## Decisión arquitectónica

**Compensate:** skill_slug or inline action por step en spec del flow.
**Order:** reverso de completion order (no de definition order).
**Failures:** persisted, no bloquean otras compensaciones.
**Status final:** ternario: failed | failed_compensated | failed_compensation_failed.

## Schema

```sql
CREATE TABLE flow_compensation_failures (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  flow_run_id UUID NOT NULL REFERENCES flow_runs(id) ON DELETE CASCADE,
  step_id VARCHAR(100) NOT NULL,
  compensate_spec JSONB NOT NULL,
  attempts INT NOT NULL DEFAULT 0,
  last_error TEXT,
  status VARCHAR(20) DEFAULT 'pending',  -- pending | skipped | resolved
  skipped_reason TEXT,
  skipped_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

ALTER TABLE flow_runs
  ADD COLUMN compensation_started_at TIMESTAMPTZ,
  ADD COLUMN compensation_finished_at TIMESTAMPTZ;
```

## Spec ejemplo

```yaml
steps:
  - id: create_user
    type: skill_call
    skill: user.create
    compensate:
      skill: user.delete
      input: { user_id: "{{ step.create_user.output.id }}" }
      retry: { max_attempts: 3, backoff: exp }
```

## TDD plan

1. 3 steps, último falla → compensa 2 anteriores reverse
2. Compensación falla → tabla failures + notif
3. Admin skip
4. Parallel mode no respeta orden
5. Idempotency
