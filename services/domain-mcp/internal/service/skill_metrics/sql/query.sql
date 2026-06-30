-- HU-52.2 — skill success rate tracking.
--
-- DEFINICION DE EXITO (conservadora, identica en todas las queries):
--   status='completed' AND output IS NOT NULL AND output <> ''
--   AND execution_time_ms > 0 AND execution_time_ms < 60000
-- DEFINICION DE FALLO:
--   status='failed'
--   OR (status='completed' AND (output IS NULL OR output='' OR execution_time_ms >= 60000))
-- Las invocaciones 'pending'/'running' NO cuentan como exito ni fallo (en vuelo).
--
-- skill_executions.created_by (UUID nullable, FK users(id) ON DELETE SET NULL,
-- agregada en 000184): caller que origino la ejecucion. unique_callers_count =
-- COUNT(DISTINCT created_by) FILTER (created_by IS NOT NULL) sobre las
-- invocaciones contables del dia. Las filas sin caller (cron/webhook de sistema
-- o ejecuciones historicas previas a 000184) tienen created_by NULL y NO suman.

-- name: AggregateDay :one
-- Computa los agregados de UN dia (UTC) para UN skill desde skill_executions.
-- p95 solo se computa si hay >= 10 invocaciones contables (data suficiente),
-- si no devuelve NULL. success_rate = success / (success+failure) * 100; NULL si
-- no hubo invocaciones contables (evita 0% espurio). avg sobre exitos+fallos.
WITH ex AS (
    SELECT
        execution_time_ms,
        created_by,
        (status = 'completed'
            AND output IS NOT NULL AND output <> ''
            AND execution_time_ms > 0 AND execution_time_ms < 60000) AS is_success,
        (status = 'failed'
            OR (status = 'completed'
                AND (output IS NULL OR output = '' OR execution_time_ms >= 60000))) AS is_failure
    FROM skill_executions
    WHERE skill_id = sqlc.arg('skill_id')
      AND (created_at AT TIME ZONE 'UTC')::date = sqlc.arg('day')::date
),
counted AS (
    SELECT
        COUNT(*) FILTER (WHERE is_success OR is_failure)::int        AS invocations_count,
        COUNT(*) FILTER (WHERE is_success)::int                      AS success_count,
        COUNT(*) FILTER (WHERE is_failure)::int                      AS failure_count,
        COUNT(DISTINCT created_by) FILTER (
            WHERE (is_success OR is_failure) AND created_by IS NOT NULL
        )::int                                                       AS unique_callers_count,
        AVG(execution_time_ms) FILTER (WHERE is_success OR is_failure) AS avg_ms,
        percentile_cont(0.95) WITHIN GROUP (
            ORDER BY execution_time_ms
        ) FILTER (WHERE is_success OR is_failure)                    AS p95_ms
    FROM ex
)
SELECT
    invocations_count,
    success_count,
    failure_count,
    CASE
        WHEN (success_count + failure_count) = 0 THEN NULL
        ELSE ROUND(success_count::numeric * 100
                   / (success_count + failure_count), 2)
    END::numeric(5,2)                                                AS success_rate,
    (CASE WHEN avg_ms IS NULL THEN NULL ELSE ROUND(avg_ms) END)::numeric AS avg_duration_ms,
    (CASE
        WHEN invocations_count < 10 THEN NULL
        WHEN p95_ms IS NULL THEN NULL
        ELSE ROUND(p95_ms)
    END)::numeric                                                   AS p95_duration_ms,
    unique_callers_count
FROM counted;

-- name: UpsertDaily :exec
-- Persiste (idempotente por PK) el agregado diario de un skill.
INSERT INTO skill_metrics_daily (
    skill_id, day, invocations_count, success_count, failure_count,
    success_rate, avg_duration_ms, p95_duration_ms, unique_callers_count, updated_at
) VALUES (
    sqlc.arg('skill_id'), sqlc.arg('day'),
    sqlc.arg('invocations_count'), sqlc.arg('success_count'), sqlc.arg('failure_count'),
    sqlc.narg('success_rate'), sqlc.narg('avg_duration_ms'), sqlc.narg('p95_duration_ms'),
    sqlc.arg('unique_callers_count'), NOW()
)
ON CONFLICT (skill_id, day) DO UPDATE
SET invocations_count    = sqlc.arg('invocations_count'),
    success_count        = sqlc.arg('success_count'),
    failure_count        = sqlc.arg('failure_count'),
    success_rate         = sqlc.narg('success_rate'),
    avg_duration_ms      = sqlc.narg('avg_duration_ms'),
    p95_duration_ms      = sqlc.narg('p95_duration_ms'),
    unique_callers_count = sqlc.arg('unique_callers_count'),
    updated_at           = NOW();

-- name: ListActiveSkillIDs :many
-- Skills vivos (no soft-deleted): el aggregator itera sobre estos cada pasada.
SELECT id
FROM skills
WHERE deleted_at IS NULL
ORDER BY id;

-- name: GetDailyBySkill :many
-- Series diarias de un skill, ultimos N dias, mas reciente primero.
SELECT skill_id, day, invocations_count, success_count, failure_count,
       success_rate::float8 AS success_rate,
       avg_duration_ms, p95_duration_ms, unique_callers_count,
       created_at, updated_at
FROM skill_metrics_daily
WHERE skill_id = sqlc.arg('skill_id')
  AND day >= (CURRENT_DATE - make_interval(days => sqlc.arg('days')::int))::date
ORDER BY day DESC;

-- name: GetDailyByDay :many
-- Todas las metricas de un dia (todos los skills), peor success_rate primero.
SELECT skill_id, day, invocations_count, success_count, failure_count,
       success_rate::float8 AS success_rate,
       avg_duration_ms, p95_duration_ms, unique_callers_count,
       created_at, updated_at
FROM skill_metrics_daily
WHERE day = sqlc.arg('day')::date
ORDER BY success_rate ASC NULLS LAST, invocations_count DESC;

-- name: ListTopFailed :many
-- Peores success_rate agregados en la ventana de N dias (solo skills con
-- invocaciones reales). Pondera por invocaciones, no por dia.
SELECT skill_id,
       SUM(invocations_count)::bigint AS invocations_count,
       SUM(success_count)::bigint     AS success_count,
       SUM(failure_count)::bigint     AS failure_count,
       CASE
           WHEN SUM(success_count) + SUM(failure_count) = 0 THEN NULL
           ELSE ROUND(SUM(success_count)::numeric * 100
                      / (SUM(success_count) + SUM(failure_count)), 2)
       END::float8 AS success_rate
FROM skill_metrics_daily
WHERE day >= (CURRENT_DATE - make_interval(days => sqlc.arg('days')::int))::date
GROUP BY skill_id
HAVING SUM(success_count) + SUM(failure_count) > 0
ORDER BY success_rate ASC NULLS LAST, invocations_count DESC
LIMIT sqlc.arg('result_limit')::int;

-- name: ListSlowest :many
-- Peor p95 en la ventana de N dias (MAX de los p95 diarios disponibles).
SELECT skill_id,
       MAX(p95_duration_ms)::int       AS p95_duration_ms,
       SUM(invocations_count)::bigint  AS invocations_count
FROM skill_metrics_daily
WHERE day >= (CURRENT_DATE - make_interval(days => sqlc.arg('days')::int))::date
  AND p95_duration_ms IS NOT NULL
GROUP BY skill_id
ORDER BY p95_duration_ms DESC
LIMIT sqlc.arg('result_limit')::int;

-- name: ListLowRateStreaks :many
-- Skills cuyos ULTIMOS 3 dias consecutivos (con datos) tienen success_rate < 70.
-- Usado por el hook de alertas. Mira solo los 3 dias mas recientes con metricas
-- por skill; si los 3 estan por debajo del umbral, el skill califica.
WITH ranked AS (
    SELECT skill_id, day, success_rate,
           ROW_NUMBER() OVER (PARTITION BY skill_id ORDER BY day DESC) AS rn
    FROM skill_metrics_daily
    WHERE success_rate IS NOT NULL
)
SELECT skill_id,
       AVG(success_rate)::float8 AS avg_success_rate,
       MIN(day)::date            AS streak_start,
       MAX(day)::date            AS streak_end
FROM ranked
WHERE rn <= 3
GROUP BY skill_id
HAVING COUNT(*) = 3
   AND MAX(success_rate) < sqlc.arg('threshold')::numeric;

-- name: RollupWeek :exec
-- Consolida las diarias de la semana de sqlc.arg('week_start') en weekly.
-- success_rate ponderado por invocaciones; avg ponderado; p95 = MAX de p95
-- diarios (aproximacion conservadora). Idempotente por PK (skill_id, week_start).
INSERT INTO skill_metrics_weekly (
    skill_id, week_start, invocations_count, success_count, failure_count,
    success_rate, avg_duration_ms, p95_duration_ms, unique_callers_count, updated_at
)
SELECT
    skill_id,
    sqlc.arg('week_start')::date AS week_start,
    SUM(invocations_count)::int  AS invocations_count,
    SUM(success_count)::int      AS success_count,
    SUM(failure_count)::int      AS failure_count,
    CASE
        WHEN SUM(success_count) + SUM(failure_count) = 0 THEN NULL
        ELSE ROUND(SUM(success_count)::numeric * 100
                   / (SUM(success_count) + SUM(failure_count)), 2)
    END::numeric(5,2)            AS success_rate,
    CASE
        WHEN SUM(invocations_count) = 0 THEN NULL
        ELSE ROUND(
            SUM(avg_duration_ms::numeric * invocations_count)
            FILTER (WHERE avg_duration_ms IS NOT NULL)
            / NULLIF(SUM(invocations_count)
                FILTER (WHERE avg_duration_ms IS NOT NULL), 0)
        )::int
    END                          AS avg_duration_ms,
    MAX(p95_duration_ms)::int    AS p95_duration_ms,
    MAX(unique_callers_count)::int AS unique_callers_count
FROM skill_metrics_daily
WHERE day >= sqlc.arg('week_start')::date
  AND day <  (sqlc.arg('week_start')::date + INTERVAL '7 days')
GROUP BY skill_id
ON CONFLICT (skill_id, week_start) DO UPDATE
SET invocations_count    = EXCLUDED.invocations_count,
    success_count        = EXCLUDED.success_count,
    failure_count        = EXCLUDED.failure_count,
    success_rate         = EXCLUDED.success_rate,
    avg_duration_ms      = EXCLUDED.avg_duration_ms,
    p95_duration_ms      = EXCLUDED.p95_duration_ms,
    unique_callers_count = EXCLUDED.unique_callers_count,
    updated_at           = NOW();

-- name: CleanupDaily :execrows
-- Retencion daily: borra filas mas viejas que N dias.
DELETE FROM skill_metrics_daily
WHERE day < (CURRENT_DATE - make_interval(days => sqlc.arg('retention_days')::int))::date;

-- name: CleanupWeekly :execrows
-- Retencion weekly: borra filas mas viejas que N dias.
DELETE FROM skill_metrics_weekly
WHERE week_start < (CURRENT_DATE - make_interval(days => sqlc.arg('retention_days')::int))::date;

-- name: ListActiveAlertsByMetric :many
-- Hook de alertas: usage_alerts activas que matchean el metric dado
-- (ej 'skill_success_rate'). Si no hay ninguna, el hook NO crea nada.
SELECT id, metric, threshold::float8 AS threshold, cooldown_secs, last_fired_at
FROM usage_alerts
WHERE active = TRUE
  AND metric = sqlc.arg('metric')::text;

-- name: InsertAlertFire :exec
-- Registra un disparo de alerta (no crea la config, solo el fire).
INSERT INTO usage_alert_fires (alert_id, metric, threshold, observed_value, payload)
VALUES (
    sqlc.arg('alert_id'),
    sqlc.arg('metric')::text,
    sqlc.arg('threshold')::float8,
    sqlc.arg('observed_value')::float8,
    sqlc.arg('payload')
);

-- name: TouchAlertFired :exec
UPDATE usage_alerts
SET last_fired_at = NOW(), fire_count = fire_count + 1
WHERE id = sqlc.arg('alert_id');
