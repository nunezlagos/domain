-- name: UpsertFeedback :one
-- Idempotente por message_id: el segundo submit del mismo mensaje hace UPDATE
-- (cambia el rating/comment/skill_slug, refresca updated_at).
INSERT INTO skill_feedback (message_id, skill_slug, rating, comment, user_email)
VALUES (
    sqlc.arg('message_id'),
    NULLIF(sqlc.arg('skill_slug'), ''),
    sqlc.arg('rating'),
    NULLIF(sqlc.arg('comment'), ''),
    NULLIF(sqlc.arg('user_email'), '')
)
ON CONFLICT (message_id) DO UPDATE
SET skill_slug = NULLIF(sqlc.arg('skill_slug'), ''),
    rating     = sqlc.arg('rating'),
    comment    = NULLIF(sqlc.arg('comment'), ''),
    user_email = NULLIF(sqlc.arg('user_email'), ''),
    updated_at = NOW()
RETURNING id, message_id,
          COALESCE(skill_slug, '')::text AS skill_slug,
          rating,
          COALESCE(comment, '')::text AS comment,
          COALESCE(user_email, '')::text AS user_email,
          created_at, updated_at;

-- name: GetFeedbackByMessage :one
SELECT id, message_id,
       COALESCE(skill_slug, '')::text AS skill_slug,
       rating,
       COALESCE(comment, '')::text AS comment,
       COALESCE(user_email, '')::text AS user_email,
       created_at, updated_at
FROM skill_feedback
WHERE message_id = sqlc.arg('message_id');

-- name: ListFeedbackBySkill :many
-- Paginado por (created_at, id) descendente. skill_slug vacio = sin filtro.
SELECT id, message_id,
       COALESCE(skill_slug, '')::text AS skill_slug,
       rating,
       COALESCE(comment, '')::text AS comment,
       COALESCE(user_email, '')::text AS user_email,
       created_at, updated_at
FROM skill_feedback
WHERE (sqlc.arg('skill_slug')::text = '' OR skill_slug = sqlc.arg('skill_slug')::text)
  AND (sqlc.arg('rating_filter')::smallint = 0 OR rating = sqlc.arg('rating_filter')::smallint)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('result_limit')::int
OFFSET sqlc.arg('result_offset')::int;

-- name: CountFeedbackBySkill :one
SELECT COUNT(*)::bigint AS total
FROM skill_feedback
WHERE (sqlc.arg('skill_slug')::text = '' OR skill_slug = sqlc.arg('skill_slug')::text)
  AND (sqlc.arg('rating_filter')::smallint = 0 OR rating = sqlc.arg('rating_filter')::smallint);

-- name: AggregateByDay :many
-- Agregados on-the-fly desde skill_feedback para los ultimos N dias.
-- Usado por el handler GET y por el aggregator cron.
SELECT COALESCE(skill_slug, '')::text AS skill_slug,
       (created_at AT TIME ZONE 'UTC')::date AS day,
       COUNT(*) FILTER (WHERE rating = 1)::int  AS count_up,
       COUNT(*) FILTER (WHERE rating = -1)::int AS count_down,
       MAX(created_at) AS last_feedback_at
FROM skill_feedback
WHERE created_at >= NOW() - make_interval(days => sqlc.arg('days')::int)
GROUP BY 1, 2
ORDER BY day DESC, skill_slug ASC;

-- name: UpsertFeedbackDaily :exec
-- Persiste un agregado diario (self-contained, NO skill_metrics).
INSERT INTO skill_feedback_daily (skill_slug, day, count_up, count_down, last_feedback_at, updated_at)
VALUES (sqlc.arg('skill_slug'), sqlc.arg('day'), sqlc.arg('count_up'),
        sqlc.arg('count_down'), sqlc.arg('last_feedback_at'), NOW())
ON CONFLICT (skill_slug, day) DO UPDATE
SET count_up         = sqlc.arg('count_up'),
    count_down       = sqlc.arg('count_down'),
    last_feedback_at = sqlc.arg('last_feedback_at'),
    updated_at       = NOW();
