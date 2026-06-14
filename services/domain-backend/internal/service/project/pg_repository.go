// Package project — pg_repository.go: implementación PG del Repository.
package project

import (
	"context"
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

func (r *pgRepository) Insert(ctx context.Context, in InsertParams) (*Project, error) {
	var p Project
	err := r.q(ctx).QueryRow(ctx,
		`INSERT INTO projects (organization_id, name, slug, description, repository_url, template_id, settings)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, organization_id, name, slug, COALESCE(description,''),
		           COALESCE(repository_url,''), template_id, settings, created_at, updated_at, deleted_at`,
		in.OrganizationID, in.Name, in.Slug, nullStr(in.Description), nullStr(in.RepositoryURL),
		in.TemplateID, in.SettingsJSON,
	).Scan(&p.ID, &p.OrganizationID, &p.Name, &p.Slug, &p.Description,
		&p.RepositoryURL, &p.TemplateID, &p.Settings, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *pgRepository) GetByID(ctx context.Context, id uuid.UUID) (*Project, error) {
	return r.queryOne(ctx, `WHERE id = $1`, id)
}

func (r *pgRepository) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Project, error) {
	return r.queryOne(ctx,
		`WHERE organization_id = $1 AND slug = $2 AND deleted_at IS NULL`, orgID, slug)
}

func (r *pgRepository) List(ctx context.Context, orgID uuid.UUID) ([]Project, error) {
	rows, err := r.q(ctx).Query(ctx,
		`SELECT id, organization_id, name, slug, COALESCE(description,''),
		        COALESCE(repository_url,''), template_id, settings, created_at, updated_at, deleted_at
		 FROM projects
		 WHERE organization_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.OrganizationID, &p.Name, &p.Slug, &p.Description,
			&p.RepositoryURL, &p.TemplateID, &p.Settings, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *pgRepository) Update(ctx context.Context, id uuid.UUID, in UpdateParams) (*Project, error) {
	var p Project
	err := r.q(ctx).QueryRow(ctx,
		`UPDATE projects
		 SET name = $2, description = $3, repository_url = $4, settings = $5
		 WHERE id = $1 AND deleted_at IS NULL
		 RETURNING id, organization_id, name, slug, COALESCE(description,''),
		           COALESCE(repository_url,''), template_id, settings, created_at, updated_at, deleted_at`,
		id, in.Name, nullStr(in.Description), nullStr(in.RepositoryURL), in.SettingsJSON,
	).Scan(&p.ID, &p.OrganizationID, &p.Name, &p.Slug, &p.Description,
		&p.RepositoryURL, &p.TemplateID, &p.Settings, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update project: %w", err)
	}
	return &p, nil
}

func (r *pgRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	_, err := r.q(ctx).Exec(ctx,
		`UPDATE projects SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	return nil
}

func (r *pgRepository) queryOne(ctx context.Context, where string, args ...any) (*Project, error) {
	var p Project
	q := `SELECT id, organization_id, name, slug, COALESCE(description,''),
	        COALESCE(repository_url,''), template_id, settings, created_at, updated_at, deleted_at
	      FROM projects ` + where
	err := r.q(ctx).QueryRow(ctx, q, args...).Scan(
		&p.ID, &p.OrganizationID, &p.Name, &p.Slug, &p.Description,
		&p.RepositoryURL, &p.TemplateID, &p.Settings, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query project: %w", err)
	}
	return &p, nil
}

// We need pgErr available for the service layer to map ErrSlugTaken in Insert.
// Aquí lo dejamos transparente: el service revisa pgerrcode.UniqueViolation.
var _ = (*pgconn.PgError)(nil)
