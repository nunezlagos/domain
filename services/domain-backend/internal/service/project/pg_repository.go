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

// selectCols centraliza el SELECT list que todas las queries leen — incluye
// el LEFT JOIN clients (REQ-28.2) para popular ClientSlug / ClientName.
const selectCols = `p.id, p.organization_id, p.name, p.slug,
		COALESCE(p.description,''),
		COALESCE(p.repository_url,''),
		p.template_id, p.settings, p.client_id,
		COALESCE(c.slug,''), COALESCE(c.name,''),
		p.created_at, p.updated_at, p.deleted_at`

const fromJoin = `FROM projects p LEFT JOIN clients c ON c.id = p.client_id`

func scanProject(row pgx.Row, p *Project) error {
	var clientSlug, clientName string
	if err := row.Scan(&p.ID, &p.OrganizationID, &p.Name, &p.Slug, &p.Description,
		&p.RepositoryURL, &p.TemplateID, &p.Settings, &p.ClientID,
		&clientSlug, &clientName,
		&p.CreatedAt, &p.UpdatedAt, &p.DeletedAt); err != nil {
		return err
	}
	p.ClientSlug = clientSlug
	p.ClientName = clientName
	return nil
}

func (r *pgRepository) Insert(ctx context.Context, in InsertParams) (*Project, error) {
	var id uuid.UUID
	// ISSUE-21.6 Fase D clean Round 3: organization_id se omite del INSERT
	// (single-org, nullable post-000145; UNIQUE (org, slug) dropeado en 000145).
	_ = in.OrganizationID
	err := r.q(ctx).QueryRow(ctx,
		`INSERT INTO projects (name, slug, description, repository_url, template_id, settings, client_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		in.Name, in.Slug, nullStr(in.Description), nullStr(in.RepositoryURL),
		in.TemplateID, in.SettingsJSON, in.ClientID,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *pgRepository) GetByID(ctx context.Context, id uuid.UUID) (*Project, error) {
	return r.queryOne(ctx, `WHERE p.id = $1`, id)
}

func (r *pgRepository) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Project, error) {
	// ISSUE-21.6 Fase D clean: single-org. WHERE sin organization_id.
	_ = orgID
	return r.queryOne(ctx,
		`WHERE p.slug = $1 AND p.deleted_at IS NULL`, slug)
}

func (r *pgRepository) List(ctx context.Context, orgID uuid.UUID, f ListFilter) ([]Project, error) {
	// ISSUE-21.6 Fase D clean: single-org, WHERE sin organization_id.
	_ = orgID
	// Construimos el WHERE dinámico — minimal para no recurrir a un builder
	// completo. Solo dos variantes: con / sin filtro de client.
	q := `SELECT ` + selectCols + ` ` + fromJoin +
		` WHERE p.deleted_at IS NULL`
	var args []any
	if f.ClientID != nil {
		q += ` AND p.client_id = $1`
		args = append(args, *f.ClientID)
	}
	q += ` ORDER BY p.created_at DESC`

	rows, err := r.q(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var p Project
		if err := scanProject(rows, &p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *pgRepository) Update(ctx context.Context, id uuid.UUID, in UpdateParams) (*Project, error) {
	// Dos paths: si ClientChanged actualiza client_id; si no, lo mantiene.
	// Mantenemos el path simple sin SQL builder porque el delta es pequeño.
	var execErr error
	if in.ClientChanged {
		_, execErr = r.q(ctx).Exec(ctx,
			`UPDATE projects
			 SET name = $2, description = $3, repository_url = $4, settings = $5, client_id = $6
			 WHERE id = $1 AND deleted_at IS NULL`,
			id, in.Name, nullStr(in.Description), nullStr(in.RepositoryURL), in.SettingsJSON, in.ClientID,
		)
	} else {
		_, execErr = r.q(ctx).Exec(ctx,
			`UPDATE projects
			 SET name = $2, description = $3, repository_url = $4, settings = $5
			 WHERE id = $1 AND deleted_at IS NULL`,
			id, in.Name, nullStr(in.Description), nullStr(in.RepositoryURL), in.SettingsJSON,
		)
	}
	if errors.Is(execErr, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if execErr != nil {
		return nil, fmt.Errorf("update project: %w", execErr)
	}
	p, err := r.GetByID(ctx, id)
	if errors.Is(err, ErrNotFound) {
		// Si el row no aparece después del UPDATE significa que no se tocó
		// (deleted_at IS NOT NULL o id inexistente).
		return nil, ErrNotFound
	}
	return p, err
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
	q := `SELECT ` + selectCols + ` ` + fromJoin + ` ` + where
	err := scanProject(r.q(ctx).QueryRow(ctx, q, args...), &p)
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
