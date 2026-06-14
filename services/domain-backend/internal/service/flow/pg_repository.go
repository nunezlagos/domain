// Package flow — pg_repository.go: implementación PG del Repository (HU-28.1).
package flow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/store/txctx"
)

type pgRepository struct {
	pool *pgxpool.Pool
}

func NewPgRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (r *pgRepository) q(ctx context.Context) querier {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return tx
	}
	return r.pool
}

func (r *pgRepository) InsertFlow(ctx context.Context, in InsertFlowParams) (*Flow, error) {
	var f Flow
	err := r.q(ctx).QueryRow(ctx,
		`INSERT INTO flows
		   (organization_id, slug, name, description, spec, deterministic_replay)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, organization_id, slug, name, COALESCE(description,''),
		           spec, is_active, deterministic_replay, seed_managed, seed_version,
		           is_user_modified, created_at, updated_at`,
		in.OrganizationID, in.Slug, in.Name, nullStr(in.Description), in.SpecJSON,
		in.DeterministicReplay,
	).Scan(&f.ID, &f.OrganizationID, &f.Slug, &f.Name, &f.Description,
		&specJSONRaw{&f.Spec}, &f.IsActive, &f.DeterministicReplay,
		&f.SeedManaged, &f.SeedVersion, &f.IsUserModified, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *pgRepository) UpdateFlow(ctx context.Context, in UpdateFlowParams) (*Flow, error) {
	where := `WHERE id = $1 AND deleted_at IS NULL`
	args := []any{in.ID, in.Name, nullStr(in.Description), in.SpecJSON, in.IsActive, in.IsUserModified}
	if in.ExpectedUpdatedAt != nil {
		where += ` AND updated_at = $7`
		args = append(args, *in.ExpectedUpdatedAt)
	}
	var f Flow
	err := r.q(ctx).QueryRow(ctx,
		`UPDATE flows SET name = $2, description = $3, spec = $4, is_active = $5,
		    is_user_modified = $6
		 `+where+`
		 RETURNING id, organization_id, slug, name, COALESCE(description,''),
		           spec, is_active, deterministic_replay, seed_managed, seed_version,
		           is_user_modified, created_at, updated_at`,
		args...,
	).Scan(&f.ID, &f.OrganizationID, &f.Slug, &f.Name, &f.Description,
		&specJSONRaw{&f.Spec}, &f.IsActive, &f.DeterministicReplay,
		&f.SeedManaged, &f.SeedVersion, &f.IsUserModified, &f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		if in.ExpectedUpdatedAt != nil {
			return nil, ErrUpdateConflict
		}
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update flow: %w", err)
	}
	return &f, nil
}

func (r *pgRepository) GetFlowByID(ctx context.Context, id uuid.UUID) (*Flow, error) {
	return r.queryOne(ctx, `WHERE id = $1 AND deleted_at IS NULL`, id)
}

func (r *pgRepository) GetFlowBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Flow, error) {
	return r.queryOne(ctx,
		`WHERE organization_id = $1 AND slug = $2 AND deleted_at IS NULL`, orgID, slug)
}

func (r *pgRepository) ListFlows(ctx context.Context, orgID uuid.UUID, limit int) ([]Flow, error) {
	rows, err := r.q(ctx).Query(ctx,
		`SELECT id, organization_id, slug, name, COALESCE(description,''),
		        spec, is_active, deterministic_replay, seed_managed, seed_version,
		        is_user_modified, created_at, updated_at
		 FROM flows WHERE organization_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at DESC LIMIT $2`, orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	defer rows.Close()
	var out []Flow
	for rows.Next() {
		var f Flow
		if err := rows.Scan(&f.ID, &f.OrganizationID, &f.Slug, &f.Name, &f.Description,
			&specJSONRaw{&f.Spec}, &f.IsActive, &f.DeterministicReplay,
			&f.SeedManaged, &f.SeedVersion, &f.IsUserModified, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (r *pgRepository) ListParents(ctx context.Context, orgID uuid.UUID, slug string) ([]Flow, error) {
	rows, err := r.q(ctx).Query(ctx, `
		SELECT id, organization_id, slug, name, COALESCE(description,''),
		       spec, is_active, deterministic_replay, seed_managed, seed_version,
		       is_user_modified, created_at, updated_at
		FROM flows
		WHERE organization_id = $1 AND deleted_at IS NULL
		  AND EXISTS (
			SELECT 1 FROM jsonb_array_elements(spec->'steps') st
			WHERE st->>'type' = 'sub_flow'
			  AND st->'config'->>'flow_slug' = $2
		  )
		ORDER BY slug ASC`, orgID, slug)
	if err != nil {
		return nil, fmt.Errorf("list parents: %w", err)
	}
	defer rows.Close()
	var out []Flow
	for rows.Next() {
		var f Flow
		if err := rows.Scan(&f.ID, &f.OrganizationID, &f.Slug, &f.Name, &f.Description,
			&specJSONRaw{&f.Spec}, &f.IsActive, &f.DeterministicReplay,
			&f.SeedManaged, &f.SeedVersion, &f.IsUserModified, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (r *pgRepository) SoftDeleteFlow(ctx context.Context, id uuid.UUID) error {
	_, err := r.q(ctx).Exec(ctx,
		`UPDATE flows SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	return nil
}

func (r *pgRepository) GetRun(ctx context.Context, id uuid.UUID) (*RunRow, error) {
	var rr RunRow
	err := r.q(ctx).QueryRow(ctx, `
		SELECT id, organization_id, flow_id, status, COALESCE(error,''),
		       started_at, finished_at, triggered_by, trigger_type
		FROM flow_runs WHERE id = $1`, id,
	).Scan(&rr.ID, &rr.OrganizationID, &rr.FlowID, &rr.Status, &rr.Error,
		&rr.StartedAt, &rr.FinishedAt, &rr.TriggeredBy, &rr.TriggerType)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRunNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get run: %w", err)
	}
	return &rr, nil
}

func (r *pgRepository) GetRunSteps(ctx context.Context, runID uuid.UUID) ([]StepRow, error) {
	rows, err := r.q(ctx).Query(ctx, `
		SELECT id, step_key, status, progress, progress_message, error, started_at, completed_at
		FROM flow_run_steps WHERE flow_run_id = $1 ORDER BY created_at ASC`, runID)
	if err != nil {
		return nil, fmt.Errorf("get run steps: %w", err)
	}
	defer rows.Close()
	var out []StepRow
	for rows.Next() {
		var st StepRow
		if err := rows.Scan(&st.ID, &st.StepKey, &st.Status, &st.Progress,
			&st.ProgressMessage, &st.Error, &st.StartedAt, &st.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan step: %w", err)
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (r *pgRepository) ListRuns(ctx context.Context, f RunFilter) ([]RunRow, int, error) {
	where := "WHERE organization_id = $1"
	args := []any{f.OrgID}
	argIdx := 2
	if f.FlowID != nil {
		where += fmt.Sprintf(" AND flow_id = $%d", argIdx)
		args = append(args, *f.FlowID)
		argIdx++
	}
	var total int
	err := r.q(ctx).QueryRow(ctx, `SELECT COUNT(*) FROM flow_runs `+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count runs: %w", err)
	}
	where += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, f.Limit, f.Offset)
	rows, err := r.q(ctx).Query(ctx, `
		SELECT id, organization_id, flow_id, status, COALESCE(error,''),
		       started_at, finished_at, triggered_by, trigger_type
		FROM flow_runs `+where, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()
	var out []RunRow
	for rows.Next() {
		var rr RunRow
		if err := rows.Scan(&rr.ID, &rr.OrganizationID, &rr.FlowID, &rr.Status, &rr.Error,
			&rr.StartedAt, &rr.FinishedAt, &rr.TriggeredBy, &rr.TriggerType); err != nil {
			return nil, 0, fmt.Errorf("scan: %w", err)
		}
		out = append(out, rr)
	}
	return out, total, rows.Err()
}

func (r *pgRepository) PauseRun(ctx context.Context, id uuid.UUID, fromStatus string) error {
	_, err := r.q(ctx).Exec(ctx,
		`UPDATE flow_runs SET status = 'paused', worker_id = NULL WHERE id = $1 AND status = $2`,
		id, fromStatus)
	if err != nil {
		return fmt.Errorf("pause run: %w", err)
	}
	return nil
}

func (r *pgRepository) ResumeRun(ctx context.Context, id uuid.UUID) error {
	_, err := r.q(ctx).Exec(ctx,
		`UPDATE flow_runs SET status = 'running' WHERE id = $1 AND (status = 'paused' OR status = 'paused_awaiting_human' OR status = 'paused_awaiting_signal' OR status = 'paused_error')`,
		id)
	if err != nil {
		return fmt.Errorf("resume run: %w", err)
	}
	return nil
}

func (r *pgRepository) CancelRun(ctx context.Context, id uuid.UUID, finishedAt time.Time) error {
	_, err := r.q(ctx).Exec(ctx,
		`UPDATE flow_runs SET status = 'cancelled', worker_id = NULL, finished_at = $1, error = COALESCE(error, '') || ' cancelled by user' WHERE id = $2`,
		finishedAt, id)
	if err != nil {
		return fmt.Errorf("cancel run: %w", err)
	}
	return nil
}

func (r *pgRepository) queryOne(ctx context.Context, where string, args ...any) (*Flow, error) {
	var f Flow
	q := `SELECT id, organization_id, slug, name, COALESCE(description,''),
	        spec, is_active, deterministic_replay, seed_managed, seed_version,
	        is_user_modified, created_at, updated_at
	      FROM flows ` + where
	err := r.q(ctx).QueryRow(ctx, q, args...).Scan(
		&f.ID, &f.OrganizationID, &f.Slug, &f.Name, &f.Description,
		&specJSONRaw{&f.Spec}, &f.IsActive, &f.DeterministicReplay,
		&f.SeedManaged, &f.SeedVersion, &f.IsUserModified, &f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	return &f, nil
}

var _ = (*pgconn.PgError)(nil)
