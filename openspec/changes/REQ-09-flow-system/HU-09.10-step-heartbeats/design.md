# Design: HU-09.10-step-heartbeats

## Decisión arquitectónica

**Throttle:** in-memory 5s min entre writes DB (configurable).
**Detector:** zombie scan cada 30s; threshold por step type.
**Transport SSE:** Postgres NOTIFY → SSE channel HU-11.3.

## Schema

```sql
ALTER TABLE flow_run_steps
  ADD COLUMN last_heartbeat_at TIMESTAMPTZ,
  ADD COLUMN progress DOUBLE PRECISION,    -- 0..1
  ADD COLUMN progress_message TEXT,
  ADD COLUMN heartbeat_threshold_seconds INT DEFAULT 120;
```

## API

```go
type ExecContext interface {
  // ...
  Heartbeat(progress float64, message string) error  // throttled internally
}
```

## TDD plan

1. Heartbeat actualiza row
2. Throttle: 10 calls en 1s → 1 write DB
3. Zombie tras 120s → failed + retry
4. SSE event publicado
5. Short step exempt (no zombie)
