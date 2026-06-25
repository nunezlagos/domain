-- name: UpsertUsageCounter :one
INSERT INTO usage_counters (period_start, tokens_used, runs_count, storage_bytes)
VALUES ($1, $2, $3, $4)
ON CONFLICT (period_start) DO UPDATE
  SET tokens_used   = usage_counters.tokens_used + EXCLUDED.tokens_used,
      runs_count    = usage_counters.runs_count + EXCLUDED.runs_count,
      storage_bytes = usage_counters.storage_bytes + EXCLUDED.storage_bytes
RETURNING period_start, tokens_used, runs_count, storage_bytes,
          cost_usd::float8 AS cost_usd, warned_80pct, warned_100pct;

-- name: GetUsageCounter :one
SELECT period_start, tokens_used, runs_count, storage_bytes,
       cost_usd::float8 AS cost_usd, warned_80pct, warned_100pct
FROM usage_counters
WHERE period_start = $1;
