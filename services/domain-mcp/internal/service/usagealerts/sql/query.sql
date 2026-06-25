-- name: InsertAlert :one
INSERT INTO usage_alerts
    (name, metric, threshold, condition, channel, recipients, cooldown_secs)
VALUES ($1, $2, sqlc.arg('threshold')::float8, $3, $4, $5, $6)
RETURNING id, created_at, updated_at;

-- name: ListAlerts :many
SELECT id, name, metric, threshold::float8 AS threshold, condition, channel,
       recipients, cooldown_secs, active, last_fired_at, fire_count,
       created_at, updated_at
FROM usage_alerts
ORDER BY created_at DESC;

-- name: GetAlert :one
SELECT id, name, metric, threshold::float8 AS threshold, condition, channel,
       recipients, cooldown_secs, active, last_fired_at, fire_count,
       created_at, updated_at
FROM usage_alerts
WHERE id = $1;

-- name: DeleteAlert :execrows
DELETE FROM usage_alerts WHERE id = $1;

-- name: SetAlertActive :execrows
UPDATE usage_alerts SET active = $2 WHERE id = $1;

-- name: UpdateAlert :one
UPDATE usage_alerts SET
    name          = COALESCE(sqlc.narg('name'), name),
    metric        = COALESCE(sqlc.narg('metric'), metric),
    threshold     = COALESCE(sqlc.narg('threshold')::float8, threshold),
    condition     = COALESCE(sqlc.narg('condition'), condition),
    channel       = COALESCE(sqlc.narg('channel'), channel),
    recipients    = COALESCE(sqlc.narg('recipients')::text[], recipients),
    cooldown_secs = COALESCE(sqlc.narg('cooldown_secs'), cooldown_secs),
    updated_at    = NOW()
WHERE id = $1
RETURNING id, name, metric, threshold::float8 AS threshold, condition, channel,
          recipients, cooldown_secs, active, last_fired_at, fire_count,
          created_at, updated_at;

-- name: ListActiveAlertsByMetrics :many
SELECT id, name, metric, threshold::float8 AS threshold, condition, channel,
       recipients, cooldown_secs, active, last_fired_at, fire_count,
       created_at, updated_at
FROM usage_alerts
WHERE active = TRUE
  AND metric = ANY($1::text[]);

-- name: ListAlertFires :many
SELECT f.id, f.alert_id, f.metric, f.threshold::float8 AS threshold,
       f.observed_value::float8 AS observed_value, f.payload, f.fired_at
FROM usage_alert_fires f
JOIN usage_alerts a ON a.id = f.alert_id
WHERE f.alert_id = $1
ORDER BY f.fired_at DESC
LIMIT $2;

-- name: InsertAlertFire :exec
INSERT INTO usage_alert_fires
    (alert_id, metric, threshold, observed_value, payload)
VALUES ($1, $2, sqlc.arg('threshold')::float8, sqlc.arg('observed_value')::float8, $3);

-- name: TouchAlertFired :exec
UPDATE usage_alerts
SET last_fired_at = NOW(), fire_count = fire_count + 1
WHERE id = $1;
