# Design: HU-09.11-reproducibility-snapshots

## Decisión arquitectónica

**Time/random:** inyectado vía ExecContext, NO `time.Now()`/`math/rand` directo en steps.
**LLM cache:** mapa por hash(model, temp, prompt) → response opcional.
**Snapshot storage:** JSONB inline si <1MB, S3 spillover si más.

## Schema

```sql
ALTER TABLE flows
  ADD COLUMN deterministic_replay BOOLEAN DEFAULT false;

ALTER TABLE flow_runs
  ADD COLUMN snapshot JSONB,
  ADD COLUMN snapshot_s3_key VARCHAR(500),
  ADD COLUMN replay_of_run_id UUID REFERENCES flow_runs(id),
  ADD COLUMN replay_mode VARCHAR(30);  -- null | deterministic | with_overrides
```

## Snapshot shape

```json
{
  "schema_version": 1,
  "flow_version_id": "...",
  "inputs": {...},
  "triggered_by": {"user_id":"...","trigger":"manual|cron|webhook"},
  "random_seed": 1234567890,
  "frozen_time_unix_ns": 1717000000000000000,
  "env_snapshot": {
    "llm_models": {"openai":"gpt-4-2024-08-06","anthropic":"claude-sonnet-4-6"}
  },
  "skill_versions": {"summarize-text":"v3","embed":"v1"},
  "llm_response_cache": {} 
}
```

## TDD plan

1. Snapshot capturado boot
2. Replay deterministic con steps puros → outputs match
3. Override modifica solo lo overridden
4. LLM cached → no llama provider
5. Opt-out reducido
6. Snapshot >1MB → S3 spillover
