package projectpolicy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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

// ISSUE-21.6: selectCols sin organization_id (Fase C).
const selectCols = `id, project_id, slug, name, kind,
		body_md, body_structured, version, is_active, override_platform,
		source, created_at, updated_at, deleted_at`

func scanPolicy(row pgx.Row) (*Policy, error) {
	var p Policy
	var structured []byte
	if err := row.Scan(
		&p.ID, &p.ProjectID, &p.Slug, &p.Name, &p.Kind,
		&p.BodyMD, &structured, &p.Version, &p.IsActive, &p.OverridePlatform,
		&p.Source, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt,
	); err != nil {
		return nil, err
	}
	if len(structured) > 0 {
		var v any
		if err := json.Unmarshal(structured, &v); err == nil {
			p.BodyStructured = v
		}
	}
	return &p, nil
}

func (r *pgRepository) Insert(ctx context.Context, in CreateInput) (*Policy, error) {
	structuredBytes := []byte("{}")
	if in.BodyStructured != nil {
		if b, err := json.Marshal(in.BodyStructured); err == nil {
			structuredBytes = b
		}
	}
	source := in.Source
	if source == "" {
		source = "manual"
	}

	row := r.q(ctx).QueryRow(ctx,
		`INSERT INTO project_policies
		   (project_id, slug, name, kind,
		    body_md, body_structured, override_platform, source)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING `+selectCols,
		in.ProjectID, in.Slug, in.Name, in.Kind,
		in.BodyMD, structuredBytes, in.OverridePlatform, source,
	)
	p, err := scanPolicy(row)
	if err != nil {
		return nil, fmt.Errorf("insert project_policy: %w", err)
	}
	return p, nil
}

func (r *pgRepository) List(ctx context.Context, orgID, projectID uuid.UUID, kind string) ([]*Policy, error) {

	_ = orgID
	q := `SELECT ` + selectCols + `
		   FROM project_policies
		   WHERE project_id = $1
		     AND is_active = TRUE AND deleted_at IS NULL AND proposed = false`
	args := []any{projectID}
	if kind != "" {
		q += " AND kind = $2"
		args = append(args, kind)
	}
	q += " ORDER BY kind ASC, slug ASC"
	rows, err := r.q(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list project_policies: %w", err)
	}
	defer rows.Close()
	out := make([]*Policy, 0)
	for rows.Next() {
		p, err := scanPolicy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *pgRepository) GetBySlug(ctx context.Context, orgID, projectID uuid.UUID, slug string) (*Policy, error) {
	_ = orgID
	row := r.q(ctx).QueryRow(ctx,
		`SELECT `+selectCols+`
		 FROM project_policies
		 WHERE project_id = $1 AND slug = $2
		   AND is_active = TRUE AND deleted_at IS NULL AND proposed = false`,
		projectID, slug,
	)
	p, err := scanPolicy(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

func (r *pgRepository) Get(ctx context.Context, orgID, id uuid.UUID) (*Policy, error) {
	_ = orgID
	row := r.q(ctx).QueryRow(ctx,
		`SELECT `+selectCols+`
		 FROM project_policies
		 WHERE id = $1 AND deleted_at IS NULL`,
		id,
	)
	p, err := scanPolicy(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

func (r *pgRepository) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, changedBy *uuid.UUID) (*Policy, error) {

	_ = orgID
	curr, err := r.Get(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		curr.Name = *in.Name
	}
	if in.Kind != nil {
		curr.Kind = *in.Kind
	}
	if in.BodyMD != nil {
		curr.BodyMD = *in.BodyMD
	}
	if in.OverridePlatform != nil {
		curr.OverridePlatform = *in.OverridePlatform
	}
	if in.BodyStructured != nil {
		curr.BodyStructured = in.BodyStructured
	}

	structuredBytes := []byte("{}")
	if curr.BodyStructured != nil {
		if b, jerr := json.Marshal(curr.BodyStructured); jerr == nil {
			structuredBytes = b
		}
	}

	newVersion := curr.Version + 1

	if _, err := r.q(ctx).Exec(ctx,
		`INSERT INTO project_policy_versions
		   (policy_id, version, body_md, body_structured, changed_by)
		 VALUES ($1,$2,$3,$4,$5)`,
		curr.ID, curr.Version, curr.BodyMD, structuredBytes, changedBy,
	); err != nil {
		return nil, fmt.Errorf("snapshot version: %w", err)
	}

	row := r.q(ctx).QueryRow(ctx,
		`UPDATE project_policies
		 SET name=$2, kind=$3, body_md=$4, body_structured=$5,
		     override_platform=$6, version=$7
		 WHERE id=$1 AND deleted_at IS NULL
		 RETURNING `+selectCols,
		id, curr.Name, curr.Kind, curr.BodyMD, structuredBytes,
		curr.OverridePlatform, newVersion,
	)
	return scanPolicy(row)
}

func (r *pgRepository) SoftDelete(ctx context.Context, orgID, id uuid.UUID) error {
	_ = orgID
	tag, err := r.q(ctx).Exec(ctx,
		`UPDATE project_policies SET deleted_at = NOW(), is_active = FALSE
		 WHERE id = $1 AND deleted_at IS NULL`,
		id,
	)
	if err != nil {
		return fmt.Errorf("soft-delete project_policy: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
