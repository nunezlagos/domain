package projectrepo

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

const selectCols = `id, organization_id, project_id, name, url,
		COALESCE(branch_default,''), COALESCE(kind,''), is_default,
		COALESCE(workflow,''), COALESCE(notes,''),
		created_at, updated_at, deleted_at`

func scanRepo(row pgx.Row) (*Repo, error) {
	var r Repo
	if err := row.Scan(
		&r.ID, &r.OrganizationID, &r.ProjectID, &r.Name, &r.URL,
		&r.BranchDefault, &r.Kind, &r.IsDefault, &r.Workflow, &r.Notes,
		&r.CreatedAt, &r.UpdatedAt, &r.DeletedAt,
	); err != nil {
		return nil, err
	}
	return &r, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (r *pgRepository) Insert(ctx context.Context, in InsertParams) (*Repo, error) {
	row := r.q(ctx).QueryRow(ctx,
		`INSERT INTO project_repositories
		   (organization_id, project_id, name, url, branch_default,
		    kind, is_default, workflow, notes)
		 VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),$7,NULLIF($8,''),NULLIF($9,''))
		 RETURNING `+selectCols,
		in.OrganizationID, in.ProjectID, in.Name, in.URL, in.BranchDefault,
		in.Kind, in.IsDefault, in.Workflow, in.Notes,
	)
	repo, err := scanRepo(row)
	if isUniqueViolation(err) {
		return nil, ErrDuplicateName
	}
	if err != nil {
		return nil, fmt.Errorf("insert project_repo: %w", err)
	}
	return repo, nil
}

func (r *pgRepository) List(ctx context.Context, orgID, projectID uuid.UUID) ([]*Repo, error) {
	rows, err := r.q(ctx).Query(ctx,
		`SELECT `+selectCols+`
		 FROM project_repositories
		 WHERE organization_id = $1 AND project_id = $2 AND deleted_at IS NULL
		 ORDER BY is_default DESC, name ASC`,
		orgID, projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list project_repos: %w", err)
	}
	defer rows.Close()
	out := make([]*Repo, 0, 4)
	for rows.Next() {
		repo, err := scanRepo(rows)
		if err != nil {
			return nil, fmt.Errorf("scan project_repo: %w", err)
		}
		out = append(out, repo)
	}
	return out, rows.Err()
}

func (r *pgRepository) Get(ctx context.Context, orgID, id uuid.UUID) (*Repo, error) {
	row := r.q(ctx).QueryRow(ctx,
		`SELECT `+selectCols+`
		 FROM project_repositories
		 WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL`,
		orgID, id,
	)
	repo, err := scanRepo(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get project_repo: %w", err)
	}
	return repo, nil
}

func (r *pgRepository) GetByName(ctx context.Context, orgID, projectID uuid.UUID, name string) (*Repo, error) {
	row := r.q(ctx).QueryRow(ctx,
		`SELECT `+selectCols+`
		 FROM project_repositories
		 WHERE organization_id = $1 AND project_id = $2 AND name = $3 AND deleted_at IS NULL`,
		orgID, projectID, name,
	)
	repo, err := scanRepo(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get project_repo by name: %w", err)
	}
	return repo, nil
}

func (r *pgRepository) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateParams) (*Repo, error) {
	// Build dinámico de SET — más simple que repetir COALESCE() para cada campo.
	sets := []string{}
	args := []any{orgID, id}
	idx := 3
	add := func(col string, v any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, idx))
		args = append(args, v)
		idx++
	}
	if in.URL != nil {
		add("url", *in.URL)
	}
	if in.BranchDefault != nil {
		add("branch_default", nullIfEmpty(*in.BranchDefault))
	}
	if in.Kind != nil {
		add("kind", nullIfEmpty(*in.Kind))
	}
	if in.Workflow != nil {
		add("workflow", nullIfEmpty(*in.Workflow))
	}
	if in.Notes != nil {
		add("notes", nullIfEmpty(*in.Notes))
	}
	if len(sets) == 0 {
		return r.Get(ctx, orgID, id)
	}
	q := `UPDATE project_repositories SET ` + joinCommas(sets) +
		` WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL
		  RETURNING ` + selectCols
	row := r.q(ctx).QueryRow(ctx, q, args...)
	repo, err := scanRepo(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update project_repo: %w", err)
	}
	return repo, nil
}

func (r *pgRepository) SetDefault(ctx context.Context, orgID, id uuid.UUID) (*Repo, error) {
	// Necesita tx para limpiar el default previo + setear el nuevo atómicamente.
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return r.setDefaultIn(ctx, tx, orgID, id)
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx for SetDefault: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	repo, err := r.setDefaultIn(ctx, tx, orgID, id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit SetDefault: %w", err)
	}
	return repo, nil
}

func (r *pgRepository) setDefaultIn(ctx context.Context, q querier, orgID, id uuid.UUID) (*Repo, error) {
	// 1. Resolver project_id
	var projectID uuid.UUID
	if err := q.QueryRow(ctx,
		`SELECT project_id FROM project_repositories
		 WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL`,
		orgID, id,
	).Scan(&projectID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("resolve project_id: %w", err)
	}
	// 2. Limpiar default actual del proyecto
	if _, err := q.Exec(ctx,
		`UPDATE project_repositories SET is_default = false
		 WHERE organization_id = $1 AND project_id = $2 AND id <> $3 AND is_default = true`,
		orgID, projectID, id,
	); err != nil {
		return nil, fmt.Errorf("clear previous default: %w", err)
	}
	// 3. Setear nuevo default
	row := q.QueryRow(ctx,
		`UPDATE project_repositories SET is_default = true
		 WHERE organization_id = $1 AND id = $2
		 RETURNING `+selectCols,
		orgID, id,
	)
	return scanRepo(row)
}

func (r *pgRepository) SoftDelete(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := r.q(ctx).Exec(ctx,
		`UPDATE project_repositories
		 SET deleted_at = NOW(), is_default = false
		 WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL`,
		orgID, id,
	)
	if err != nil {
		return fmt.Errorf("soft-delete project_repo: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func joinCommas(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
