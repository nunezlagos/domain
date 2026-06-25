-- name: GetSourceProject :one
SELECT deleted_at, slug FROM projects WHERE id = @source_id;

-- name: CheckTargetExists :one
SELECT 1 FROM projects WHERE id = @target_id AND deleted_at IS NULL;

-- name: MoveObservations :execrows
UPDATE knowledge_observations SET project_id = @target_id WHERE project_id = @source_id AND deleted_at IS NULL;

-- name: SoftDeleteProject :exec
UPDATE projects SET deleted_at = now(), updated_at = now() WHERE id = @source_id;

-- name: InsertMergeRecord :exec
INSERT INTO project_merges (id, source_id, target_id, actor_id, report, merged_at)
VALUES (@id, @source_id, @target_id, @actor_id, @report, now());

-- name: InsertMergeRecordNoActor :exec
INSERT INTO project_merges (id, source_id, target_id, merged_at)
VALUES (@id, @source_id, @target_id, now());
