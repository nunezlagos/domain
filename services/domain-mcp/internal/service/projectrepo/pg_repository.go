package projectrepo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/projectrepo/projectrepodb"
	"nunezlagos/domain/internal/store/txctx"
)

type pgRepository struct {
	pool *pgxpool.Pool
}

func NewPgRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) q(ctx context.Context) *projectrepodb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return projectrepodb.New(tx)
	}
	return projectrepodb.New(r.pool)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func deletedAtPtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}

func (r *pgRepository) Insert(ctx context.Context, in InsertParams) (*Repo, error) {
	row, err := r.q(ctx).InsertRepo(ctx, projectrepodb.InsertRepoParams{
		ProjectID:     in.ProjectID,
		Name:          in.Name,
		Url:           in.URL,
		BranchDefault: in.BranchDefault,
		Kind:          in.Kind,
		IsDefault:     in.IsDefault,
		Workflow:      in.Workflow,
		Notes:         in.Notes,
		RootPath:      in.RootPath,
	})
	if isUniqueViolation(err) {
		return nil, ErrDuplicateName
	}
	if err != nil {
		return nil, fmt.Errorf("insert project_repo: %w", err)
	}
	return &Repo{
		ID:            row.ID,
		ProjectID:     row.ProjectID,
		Name:          row.Name,
		URL:           row.Url,
		BranchDefault: row.BranchDefault,
		Kind:          row.Kind,
		IsDefault:     row.IsDefault,
		Workflow:      row.Workflow,
		Notes:         row.Notes,
		RootPath:      row.RootPath,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
		DeletedAt:     deletedAtPtr(row.DeletedAt),
	}, nil
}

func (r *pgRepository) List(ctx context.Context, orgID, projectID uuid.UUID) ([]*Repo, error) {
	rows, err := r.q(ctx).ListReposByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project_repos: %w", err)
	}
	out := make([]*Repo, 0, len(rows))
	for _, row := range rows {
		out = append(out, &Repo{
			ID:            row.ID,
			ProjectID:     row.ProjectID,
			Name:          row.Name,
			URL:           row.Url,
			BranchDefault: row.BranchDefault,
			Kind:          row.Kind,
			IsDefault:     row.IsDefault,
			Workflow:      row.Workflow,
			Notes:         row.Notes,
			RootPath:      row.RootPath,
			CreatedAt:     row.CreatedAt,
			UpdatedAt:     row.UpdatedAt,
			DeletedAt:     deletedAtPtr(row.DeletedAt),
		})
	}
	return out, nil
}

func (r *pgRepository) Get(ctx context.Context, orgID, id uuid.UUID) (*Repo, error) {
	row, err := r.q(ctx).GetRepoByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get project_repo: %w", err)
	}
	return &Repo{
		ID:            row.ID,
		ProjectID:     row.ProjectID,
		Name:          row.Name,
		URL:           row.Url,
		BranchDefault: row.BranchDefault,
		Kind:          row.Kind,
		IsDefault:     row.IsDefault,
		Workflow:      row.Workflow,
		Notes:         row.Notes,
		RootPath:      row.RootPath,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
		DeletedAt:     deletedAtPtr(row.DeletedAt),
	}, nil
}

func (r *pgRepository) GetByName(ctx context.Context, orgID, projectID uuid.UUID, name string) (*Repo, error) {
	row, err := r.q(ctx).GetRepoByName(ctx, projectrepodb.GetRepoByNameParams{
		ProjectID: projectID,
		Name:      name,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get project_repo by name: %w", err)
	}
	return &Repo{
		ID:            row.ID,
		ProjectID:     row.ProjectID,
		Name:          row.Name,
		URL:           row.Url,
		BranchDefault: row.BranchDefault,
		Kind:          row.Kind,
		IsDefault:     row.IsDefault,
		Workflow:      row.Workflow,
		Notes:         row.Notes,
		RootPath:      row.RootPath,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
		DeletedAt:     deletedAtPtr(row.DeletedAt),
	}, nil
}

func (r *pgRepository) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateParams) (*Repo, error) {
	if in.URL == nil && in.BranchDefault == nil && in.Kind == nil &&
		in.Workflow == nil && in.Notes == nil && in.RootPath == nil {
		return r.Get(ctx, orgID, id)
	}
	row, err := r.q(ctx).UpdateRepo(ctx, projectrepodb.UpdateRepoParams{
		ID:            id,
		Url:           in.URL,
		BranchDefault: in.BranchDefault,
		Kind:          in.Kind,
		Workflow:      in.Workflow,
		Notes:         in.Notes,
		RootPath:      in.RootPath,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update project_repo: %w", err)
	}
	return &Repo{
		ID:            row.ID,
		ProjectID:     row.ProjectID,
		Name:          row.Name,
		URL:           row.Url,
		BranchDefault: row.BranchDefault,
		Kind:          row.Kind,
		IsDefault:     row.IsDefault,
		Workflow:      row.Workflow,
		Notes:         row.Notes,
		RootPath:      row.RootPath,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
		DeletedAt:     deletedAtPtr(row.DeletedAt),
	}, nil
}

func (r *pgRepository) SetDefault(ctx context.Context, orgID, id uuid.UUID) (*Repo, error) {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return r.setDefaultIn(ctx, projectrepodb.New(tx), id)
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx for SetDefault: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	repo, err := r.setDefaultIn(ctx, projectrepodb.New(tx), id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit SetDefault: %w", err)
	}
	return repo, nil
}

func (r *pgRepository) setDefaultIn(ctx context.Context, q *projectrepodb.Queries, id uuid.UUID) (*Repo, error) {
	projectID, err := q.GetRepoProjectID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("resolve project_id: %w", err)
	}

	if err := q.ClearProjectDefault(ctx, projectrepodb.ClearProjectDefaultParams{
		ProjectID: projectID,
		ID:        id,
	}); err != nil {
		return nil, fmt.Errorf("clear previous default: %w", err)
	}

	row, err := q.SetRepoAsDefault(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("set default: %w", err)
	}
	return &Repo{
		ID:            row.ID,
		ProjectID:     row.ProjectID,
		Name:          row.Name,
		URL:           row.Url,
		BranchDefault: row.BranchDefault,
		Kind:          row.Kind,
		IsDefault:     row.IsDefault,
		Workflow:      row.Workflow,
		Notes:         row.Notes,
		RootPath:      row.RootPath,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
		DeletedAt:     deletedAtPtr(row.DeletedAt),
	}, nil
}

func (r *pgRepository) SoftDelete(ctx context.Context, orgID, id uuid.UUID) error {
	n, err := r.q(ctx).SoftDeleteRepo(ctx, id)
	if err != nil {
		return fmt.Errorf("soft-delete project_repo: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
