// Package agent — pg_repository.go: implementación PG del Repository.
package agent

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

func (r *pgRepository) Insert(ctx context.Context, in InsertParams) (*Agent, error) {
	var a Agent
	err := r.q(ctx).QueryRow(ctx,
		`INSERT INTO agents
		   (organization_id, slug, name, description, provider, model, system_prompt,
		    skills_slugs, max_iterations, token_budget, temperature)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id, organization_id, slug, name, COALESCE(description,''),
		           provider, model, COALESCE(system_prompt,''), skills_slugs,
		           max_iterations, token_budget, temperature,
		           seed_managed, seed_version, is_user_modified, created_at, updated_at`,
		in.OrganizationID, in.Slug, in.Name, nullStr(in.Description), in.Provider, in.Model,
		nullStr(in.SystemPrompt), in.SkillsSlugs, in.MaxIterations, in.TokenBudget, in.Temperature,
	).Scan(&a.ID, &a.OrganizationID, &a.Slug, &a.Name, &a.Description,
		&a.Provider, &a.Model, &a.SystemPrompt, &a.SkillsSlugs,
		&a.MaxIterations, &a.TokenBudget, &a.Temperature,
		&a.SeedManaged, &a.SeedVersion, &a.IsUserModified, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *pgRepository) Update(ctx context.Context, id uuid.UUID, in UpdateParams) (*Agent, error) {
	var a Agent
	err := r.q(ctx).QueryRow(ctx,
		`UPDATE agents
		 SET name = $2, description = $3, provider = $4, model = $5,
		     system_prompt = $6, skills_slugs = $7, max_iterations = $8,
		     token_budget = $9, temperature = $10, is_user_modified = $11
		 WHERE id = $1 AND deleted_at IS NULL
		 RETURNING id, organization_id, slug, name, COALESCE(description,''),
		           provider, model, COALESCE(system_prompt,''), skills_slugs,
		           max_iterations, token_budget, temperature,
		           seed_managed, seed_version, is_user_modified, created_at, updated_at`,
		id, in.Name, nullStr(in.Description), in.Provider, in.Model, nullStr(in.SystemPrompt),
		in.SkillsSlugs, in.MaxIterations, in.TokenBudget, in.Temperature, in.IsUserModified,
	).Scan(&a.ID, &a.OrganizationID, &a.Slug, &a.Name, &a.Description,
		&a.Provider, &a.Model, &a.SystemPrompt, &a.SkillsSlugs,
		&a.MaxIterations, &a.TokenBudget, &a.Temperature,
		&a.SeedManaged, &a.SeedVersion, &a.IsUserModified, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update agent: %w", err)
	}
	return &a, nil
}

func (r *pgRepository) GetByID(ctx context.Context, id uuid.UUID) (*Agent, error) {
	return r.queryOne(ctx, `WHERE id = $1 AND deleted_at IS NULL`, id)
}

func (r *pgRepository) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Agent, error) {
	return r.queryOne(ctx,
		`WHERE organization_id = $1 AND slug = $2 AND deleted_at IS NULL`, orgID, slug)
}

func (r *pgRepository) List(ctx context.Context, orgID uuid.UUID, limit int) ([]Agent, error) {
	rows, err := r.q(ctx).Query(ctx,
		`SELECT id, organization_id, slug, name, COALESCE(description,''),
		        provider, model, COALESCE(system_prompt,''), skills_slugs,
		        max_iterations, token_budget, temperature,
		        seed_managed, seed_version, is_user_modified, created_at, updated_at
		 FROM agents
		 WHERE organization_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at DESC LIMIT $2`, orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	defer rows.Close()
	var out []Agent
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.ID, &a.OrganizationID, &a.Slug, &a.Name, &a.Description,
			&a.Provider, &a.Model, &a.SystemPrompt, &a.SkillsSlugs,
			&a.MaxIterations, &a.TokenBudget, &a.Temperature,
			&a.SeedManaged, &a.SeedVersion, &a.IsUserModified, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *pgRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	_, err := r.q(ctx).Exec(ctx,
		`UPDATE agents SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	return nil
}

func (r *pgRepository) CountValidSkills(ctx context.Context, orgID uuid.UUID, slugs []string) (int, error) {
	var foundCount int
	err := r.q(ctx).QueryRow(ctx,
		`SELECT COUNT(*) FROM skills
		 WHERE organization_id = $1 AND slug = ANY($2) AND deleted_at IS NULL`,
		orgID, slugs,
	).Scan(&foundCount)
	if err != nil {
		return 0, fmt.Errorf("validate skills: %w", err)
	}
	return foundCount, nil
}

func (r *pgRepository) ModelExists(ctx context.Context, provider, model string) (bool, error) {
	var exists bool
	err := r.q(ctx).QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM model_registry
		   WHERE provider = $1 AND model = $2 AND modality = 'completion' AND is_active = TRUE)`,
		provider, model).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("validate model: %w", err)
	}
	return exists, nil
}

func (r *pgRepository) SlugTaken(ctx context.Context, orgID uuid.UUID, slug string) (bool, error) {
	var taken bool
	err := r.q(ctx).QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM agents
		 WHERE organization_id = $1 AND slug = $2 AND deleted_at IS NULL)`,
		orgID, slug).Scan(&taken)
	if err != nil {
		return false, fmt.Errorf("check slug: %w", err)
	}
	return taken, nil
}

func (r *pgRepository) ArchiveVersion(ctx context.Context, in ArchiveVersionParams) error {
	var changedBy any
	if in.ChangedBy != nil && *in.ChangedBy != uuid.Nil {
		changedBy = *in.ChangedBy
	}
	_, err := r.q(ctx).Exec(ctx,
		`INSERT INTO agent_versions (agent_id, version, snapshot, changed_by)
		 SELECT $1, COALESCE(MAX(version), 0) + 1, $2, $3
		 FROM agent_versions WHERE agent_id = $1`,
		in.AgentID, in.Snapshot, changedBy)
	if err != nil {
		return fmt.Errorf("archive agent version: %w", err)
	}
	_, err = r.q(ctx).Exec(ctx,
		`DELETE FROM agent_versions
		 WHERE agent_id = $1 AND version <= (
		   SELECT MAX(version) - $2 FROM agent_versions WHERE agent_id = $1)`,
		in.AgentID, in.MaxVersionsKept)
	if err != nil {
		return fmt.Errorf("purge agent versions: %w", err)
	}
	return nil
}

func (r *pgRepository) ListVersions(ctx context.Context, agentID uuid.UUID, limit int) ([]AgentVersion, error) {
	rows, err := r.q(ctx).Query(ctx,
		`SELECT version, snapshot, changed_by, created_at
		 FROM agent_versions WHERE agent_id = $1
		 ORDER BY version DESC LIMIT $2`, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()
	var out []AgentVersion
	for rows.Next() {
		var v AgentVersion
		if err := rows.Scan(&v.Version, &v.Snapshot, &v.ChangedBy, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *pgRepository) queryOne(ctx context.Context, where string, args ...any) (*Agent, error) {
	var a Agent
	q := `SELECT id, organization_id, slug, name, COALESCE(description,''),
	        provider, model, COALESCE(system_prompt,''), skills_slugs,
	        max_iterations, token_budget, temperature,
	        seed_managed, seed_version, is_user_modified, created_at, updated_at
	      FROM agents ` + where
	err := r.q(ctx).QueryRow(ctx, q, args...).Scan(
		&a.ID, &a.OrganizationID, &a.Slug, &a.Name, &a.Description,
		&a.Provider, &a.Model, &a.SystemPrompt, &a.SkillsSlugs,
		&a.MaxIterations, &a.TokenBudget, &a.Temperature,
		&a.SeedManaged, &a.SeedVersion, &a.IsUserModified, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	return &a, nil
}

var _ = (*pgconn.PgError)(nil)
