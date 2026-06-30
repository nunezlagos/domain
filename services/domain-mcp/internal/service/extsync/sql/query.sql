-- name: RegisterProvider :one
INSERT INTO external_providers (provider, display_name, base_url, project_key, config)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (provider, project_key)
DO UPDATE SET display_name = EXCLUDED.display_name,
              base_url = EXCLUDED.base_url,
              config = EXCLUDED.config,
              updated_at = now()
RETURNING id, provider, display_name, base_url, project_key,
          config, enabled, created_at, updated_at;

-- name: RegisterPush :one
INSERT INTO external_sync_state (provider_id, entity_kind, entity_id, external_key,
                                  external_url, external_type, sync_direction,
                                  sync_status, field_mapping, last_pushed_at,
                                  last_synced_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now(), now())
RETURNING *;

-- name: MarkDrift :exec
UPDATE external_sync_state
SET sync_status = $1, drift_detected_at = now(), drift_fields = $2,
    updated_at = now()
WHERE id = $3;

-- name: MarkPartial :exec
UPDATE external_sync_state
SET sync_status = $1, partial_failures = $2, updated_at = now()
WHERE id = $3;

-- name: MarkResolved :exec
UPDATE external_sync_state
SET sync_status = $1, drift_detected_at = NULL, drift_fields = NULL,
    last_synced_at = now(), updated_at = now()
WHERE id = $2;

-- name: GetSyncState :one
SELECT * FROM external_sync_state WHERE id = $1;

-- name: GetByEntity :one
SELECT * FROM external_sync_state
WHERE provider_id = $1 AND entity_kind = $2 AND entity_id = $3;

-- name: ListConflicts :many
SELECT * FROM external_sync_state
WHERE sync_status = $1
ORDER BY drift_detected_at DESC NULLS LAST LIMIT $2;

-- name: InsertSyncEvent :exec
INSERT INTO external_sync_events (sync_state_id, event_type, direction, payload, error_message)
VALUES ($1, $2, $3, $4, $5);
