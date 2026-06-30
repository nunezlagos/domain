// Package agent — pg_repository.go: implementación PG del Repository via sqlc.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/agent/agentdb"
	"nunezlagos/domain/internal/store/txctx"
)

type pgRepository struct {
	pool *pgxpool.Pool
}

func NewPgRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) q(ctx context.Context) *agentdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return agentdb.New(tx)
	}
	return agentdb.New(r.pool)
}

// numericFromFloat64 convierte *float64 a pgtype.Numeric para sqlc.
func numericFromFloat64(f *float64) pgtype.Numeric {
	if f == nil {
		return pgtype.Numeric{Valid: false}
	}
	var n pgtype.Numeric
	_ = n.Scan(*f)
	return n
}

// numericToFloat64Ptr convierte pgtype.Numeric a *float64.
func numericToFloat64Ptr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	f, _ := n.Float64Value()
	if !f.Valid {
		return nil
	}
	v := f.Float64
	return &v
}

// rowToAgent convierte los campos comunes de las rows generadas por sqlc a Agent.
func rowToAgent(
	id uuid.UUID, slug, name, description, provider, model, systemPrompt string,
	skillsSlugs []string, maxIterations int32, tokenBudget *int64,
	temperature pgtype.Numeric, seedManaged bool, seedVersion *int32,
	isUserModified bool, createdAt, updatedAt interface{},
) Agent {
	var sv *int
	if seedVersion != nil {
		v := int(*seedVersion)
		sv = &v
	}
	a := Agent{
		ID:             id,
		Slug:           slug,
		Name:           name,
		Description:    description,
		Provider:       provider,
		Model:          model,
		SystemPrompt:   systemPrompt,
		SkillsSlugs:    skillsSlugs,
		MaxIterations:  int(maxIterations),
		TokenBudget:    tokenBudget,
		Temperature:    numericToFloat64Ptr(temperature),
		SeedManaged:    seedManaged,
		SeedVersion:    sv,
		IsUserModified: isUserModified,
	}
	return a
}

func (r *pgRepository) Insert(ctx context.Context, in InsertParams) (*Agent, error) {
	var desc *string
	if in.Description != "" {
		s := in.Description
		desc = &s
	}
	var sp *string
	if in.SystemPrompt != "" {
		s := in.SystemPrompt
		sp = &s
	}
	row, err := r.q(ctx).InsertAgent(ctx, agentdb.InsertAgentParams{
		Slug:          in.Slug,
		Name:          in.Name,
		Description:   desc,
		Provider:      in.Provider,
		Model:         in.Model,
		SystemPrompt:  sp,
		SkillsSlugs:   in.SkillsSlugs,
		MaxIterations: int32(in.MaxIterations),
		TokenBudget:   in.TokenBudget,
		Temperature:   numericFromFloat64(in.Temperature),
	})
	if err != nil {
		return nil, err
	}
	a := rowToAgent(row.ID, row.Slug, row.Name, row.Description,
		row.Provider, row.Model, row.SystemPrompt, row.SkillsSlugs,
		row.MaxIterations, row.TokenBudget, row.Temperature,
		row.SeedManaged, row.SeedVersion, row.IsUserModified,
		row.CreatedAt, row.UpdatedAt)
	a.CreatedAt = row.CreatedAt
	a.UpdatedAt = row.UpdatedAt
	return &a, nil
}

func (r *pgRepository) Update(ctx context.Context, id uuid.UUID, in UpdateParams) (*Agent, error) {
	var desc *string
	if in.Description != "" {
		s := in.Description
		desc = &s
	}
	var sp *string
	if in.SystemPrompt != "" {
		s := in.SystemPrompt
		sp = &s
	}
	row, err := r.q(ctx).UpdateAgent(ctx, agentdb.UpdateAgentParams{
		ID:             id,
		Name:           in.Name,
		Description:    desc,
		Provider:       in.Provider,
		Model:          in.Model,
		SystemPrompt:   sp,
		SkillsSlugs:    in.SkillsSlugs,
		MaxIterations:  int32(in.MaxIterations),
		TokenBudget:    in.TokenBudget,
		Temperature:    numericFromFloat64(in.Temperature),
		IsUserModified: in.IsUserModified,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update agent: %w", err)
	}
	a := rowToAgent(row.ID, row.Slug, row.Name, row.Description,
		row.Provider, row.Model, row.SystemPrompt, row.SkillsSlugs,
		row.MaxIterations, row.TokenBudget, row.Temperature,
		row.SeedManaged, row.SeedVersion, row.IsUserModified,
		row.CreatedAt, row.UpdatedAt)
	a.CreatedAt = row.CreatedAt
	a.UpdatedAt = row.UpdatedAt
	return &a, nil
}

func (r *pgRepository) GetByID(ctx context.Context, id uuid.UUID) (*Agent, error) {
	row, err := r.q(ctx).GetAgentByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get agent by id: %w", err)
	}
	a := rowToAgent(row.ID, row.Slug, row.Name, row.Description,
		row.Provider, row.Model, row.SystemPrompt, row.SkillsSlugs,
		row.MaxIterations, row.TokenBudget, row.Temperature,
		row.SeedManaged, row.SeedVersion, row.IsUserModified,
		row.CreatedAt, row.UpdatedAt)
	a.CreatedAt = row.CreatedAt
	a.UpdatedAt = row.UpdatedAt
	return &a, nil
}

func (r *pgRepository) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Agent, error) {
	row, err := r.q(ctx).GetAgentBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get agent by slug: %w", err)
	}
	a := rowToAgent(row.ID, row.Slug, row.Name, row.Description,
		row.Provider, row.Model, row.SystemPrompt, row.SkillsSlugs,
		row.MaxIterations, row.TokenBudget, row.Temperature,
		row.SeedManaged, row.SeedVersion, row.IsUserModified,
		row.CreatedAt, row.UpdatedAt)
	a.CreatedAt = row.CreatedAt
	a.UpdatedAt = row.UpdatedAt
	return &a, nil
}

func (r *pgRepository) List(ctx context.Context, orgID uuid.UUID, limit int) ([]Agent, error) {
	rows, err := r.q(ctx).ListAgents(ctx, int32(limit))
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	out := make([]Agent, 0, len(rows))
	for _, row := range rows {
		a := rowToAgent(row.ID, row.Slug, row.Name, row.Description,
			row.Provider, row.Model, row.SystemPrompt, row.SkillsSlugs,
			row.MaxIterations, row.TokenBudget, row.Temperature,
			row.SeedManaged, row.SeedVersion, row.IsUserModified,
			row.CreatedAt, row.UpdatedAt)
		a.CreatedAt = row.CreatedAt
		a.UpdatedAt = row.UpdatedAt
		out = append(out, a)
	}
	return out, nil
}

func (r *pgRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if err := r.q(ctx).SoftDeleteAgent(ctx, id); err != nil {
		return fmt.Errorf("soft delete agent: %w", err)
	}
	return nil
}

func (r *pgRepository) CountValidSkills(ctx context.Context, orgID uuid.UUID, slugs []string) (int, error) {
	count, err := r.q(ctx).CountValidSkills(ctx, slugs)
	if err != nil {
		return 0, fmt.Errorf("validate skills: %w", err)
	}
	return int(count), nil
}

func (r *pgRepository) SlugTaken(ctx context.Context, orgID uuid.UUID, slug string) (bool, error) {
	taken, err := r.q(ctx).AgentSlugTaken(ctx, slug)
	if err != nil {
		return false, fmt.Errorf("check slug: %w", err)
	}
	return taken, nil
}

func (r *pgRepository) ArchiveVersion(ctx context.Context, in ArchiveVersionParams) error {
	snapshot, err := json.Marshal(in.Snapshot)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	var changedBy *uuid.UUID
	if in.ChangedBy != nil && *in.ChangedBy != uuid.Nil {
		changedBy = in.ChangedBy
	}
	if err := r.q(ctx).InsertAgentVersion(ctx, agentdb.InsertAgentVersionParams{
		AgentID:   in.AgentID,
		Snapshot:  snapshot,
		ChangedBy: changedBy,
	}); err != nil {
		return fmt.Errorf("archive agent version: %w", err)
	}
	if err := r.q(ctx).PurgeOldAgentVersions(ctx, agentdb.PurgeOldAgentVersionsParams{
		AgentID:         in.AgentID,
		MaxVersionsKept: int32(in.MaxVersionsKept),
	}); err != nil {
		return fmt.Errorf("purge agent versions: %w", err)
	}
	return nil
}

func (r *pgRepository) ListVersions(ctx context.Context, agentID uuid.UUID, limit int) ([]AgentVersion, error) {
	rows, err := r.q(ctx).ListAgentVersions(ctx, agentdb.ListAgentVersionsParams{
		AgentID:     agentID,
		ResultLimit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list agent versions: %w", err)
	}
	out := make([]AgentVersion, 0, len(rows))
	for _, row := range rows {
		var snap map[string]any
		if err := json.Unmarshal(row.Snapshot, &snap); err != nil {
			return nil, fmt.Errorf("unmarshal snapshot: %w", err)
		}
		out = append(out, AgentVersion{
			Version:   int(row.Version),
			Snapshot:  snap,
			ChangedBy: row.ChangedBy,
			CreatedAt: row.CreatedAt,
		})
	}
	return out, nil
}
