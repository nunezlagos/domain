-- name: GetCounters :one
SELECT
  (SELECT COUNT(*)::bigint FROM knowledge_observations
     WHERE knowledge_observations.created_at >= $1 AND knowledge_observations.created_at < $2 AND knowledge_observations.deleted_at IS NULL) AS observations,
  (SELECT COUNT(*)::bigint FROM agents
     WHERE agents.deleted_at IS NULL) AS agents,
  (SELECT COUNT(*)::bigint FROM agent_runs
     WHERE agent_runs.created_at >= $1 AND agent_runs.created_at < $2) AS agent_runs,
  (SELECT COUNT(*)::bigint FROM flow_runs
     WHERE flow_runs.created_at >= $1 AND flow_runs.created_at < $2) AS flow_runs;

-- name: GetMaxDuration :one
SELECT max_flow_duration_seconds FROM flow_config LIMIT 1;

-- name: GetRunHistory :many
WITH series AS (
  SELECT generate_series($1::timestamptz, $2::timestamptz - interval '1 day', interval '1 day')::date AS day
),
agent_runs_agg AS (
  SELECT date_trunc('day', created_at AT TIME ZONE 'UTC')::date AS day,
         COUNT(*)::bigint AS count
  FROM agent_runs
  WHERE created_at >= $1 AND created_at < $2
  GROUP BY 1
),
flow_runs_agg AS (
  SELECT date_trunc('day', created_at AT TIME ZONE 'UTC')::date AS day,
         COUNT(*)::bigint AS count
  FROM flow_runs
  WHERE created_at >= $1 AND created_at < $2
  GROUP BY 1
)
SELECT s.day,
       COALESCE(a.count, 0)::bigint AS agent_runs,
       COALESCE(f.count, 0)::bigint AS flow_runs
FROM series s
LEFT JOIN agent_runs_agg a ON a.day = s.day
LEFT JOIN flow_runs_agg f ON f.day = s.day
ORDER BY s.day DESC;

-- name: GetObservationHistory :many
SELECT date_trunc('day', created_at AT TIME ZONE 'UTC')::date AS day,
       COUNT(*)::bigint AS count
FROM knowledge_observations
WHERE created_at >= $1 AND created_at < $2 AND deleted_at IS NULL
GROUP BY 1;
