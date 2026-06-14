# Design: issue-09.6-durable-execution

## Decisión arquitectónica

**Worker claim:** atomic UPDATE WHERE worker_id IS NULL.
**Heartbeat:** 30s interval, threshold 60s for recovery.
**Checkpoint:** per step en tx + commit antes de iniciar next.
**Output overflow:** gzip + S3 spillover si >10MB.

## Schema

```sql
ALTER TABLE flow_runs
  ADD COLUMN worker_id VARCHAR(64),
  ADD COLUMN last_heartbeat_at TIMESTAMPTZ,
  ADD COLUMN cursor JSONB DEFAULT '{}',
  ADD COLUMN recovery_count INT DEFAULT 0;
CREATE INDEX ON flow_runs (status, last_heartbeat_at) WHERE status = 'running';
CREATE INDEX ON flow_runs (status, worker_id) WHERE status IN ('pending','running');

CREATE TABLE flow_run_steps (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  flow_run_id UUID NOT NULL REFERENCES flow_runs(id) ON DELETE CASCADE,
  step_id VARCHAR(100) NOT NULL,
  step_type VARCHAR(50) NOT NULL,
  attempt INT NOT NULL DEFAULT 1,
  status VARCHAR(20) NOT NULL,       -- running | completed | failed | skipped
  input_compressed BYTEA,
  output_compressed BYTEA,
  output_s3_key VARCHAR(500),
  error TEXT,
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  replay_safe BOOLEAN DEFAULT true,
  UNIQUE (flow_run_id, step_id, attempt)
);
CREATE INDEX ON flow_run_steps (flow_run_id, started_at);
```

## Worker lifecycle

```go
for {
  run := ClaimNextRun()  // UPDATE ... SET worker_id WHERE worker_id IS NULL RETURNING
  if run == nil { sleep(1s); continue }
  
  hbCtx, cancel := context.WithCancel(ctx)
  go heartbeat(hbCtx, run.ID, 30*time.Second)
  
  err := ExecuteResume(ctx, run)  // resume from run.cursor
  
  cancel()
  if err != nil { MarkFailed(run, err) } else { MarkCompleted(run) }
}

func recoveryScanner() {
  for { 
    sleep(60s)
    UPDATE flow_runs SET worker_id = NULL, recovery_count = recovery_count + 1
      WHERE status = 'running' AND last_heartbeat_at < now() - interval '60s'
  }
}
```

## TDD plan

1. Step output checkpointed entre A y B
2. Resume desde step 3 sin re-correr 1 y 2
3. Crash sim: kill worker mid → scanner reasigna
4. Heartbeat actualiza
5. Replay-unsafe → pausa awaiting_human
6. Idempotency key estable cross-attempt
7. Output 15MB → S3 spillover
8. Race 2 workers claim → solo 1 gana
9. Recovery_count incrementa
