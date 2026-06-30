package orchestration

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

// TemplateStore — issue-08.5 CRUD de agent_templates.
type TemplateStore struct {
	Pool *pgxpool.Pool
}

var ErrTemplateNotFound = errors.New("agent template not found")

func (s *TemplateStore) q(ctx context.Context) *agentdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return agentdb.New(tx)
	}
	return agentdb.New(s.Pool)
}

func numericFromFloat32(f float32) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(float64(f))
	return n
}

func numericToFloat32(n pgtype.Numeric) float32 {
	if !n.Valid {
		return 0
	}
	f, _ := n.Float64Value()
	if !f.Valid {
		return 0
	}
	return float32(f.Float64)
}

func rowToTemplate(
	id uuid.UUID, slug, name, systemPrompt, personality string,
	capabilities []string, model string, temperature pgtype.Numeric,
	maxTokens int32, handoffPolicy string, metadata []byte,
) AgentTemplate {
	return AgentTemplate{
		ID:            id,
		Slug:          slug,
		Name:          name,
		SystemPrompt:  systemPrompt,
		Personality:   personality,
		Capabilities:  capabilities,
		Model:         model,
		Temperature:   numericToFloat32(temperature),
		MaxTokens:     int(maxTokens),
		HandoffPolicy: handoffPolicy,
		Metadata:      json.RawMessage(metadata),
	}
}

// Upsert crea o actualiza por (org, slug). Devuelve la versión persistida.
func (s *TemplateStore) Upsert(ctx context.Context, orgID uuid.UUID, t AgentTemplate) (*AgentTemplate, error) {
	if t.HandoffPolicy == "" {
		t.HandoffPolicy = HandoffAllow
	}
	meta := t.Metadata
	if len(meta) == 0 {
		meta = json.RawMessage("{}")
	}
	row, err := s.q(ctx).UpsertAgentTemplate(ctx, agentdb.UpsertAgentTemplateParams{
		Slug:          t.Slug,
		Name:          t.Name,
		SystemPrompt:  t.SystemPrompt,
		Personality:   t.Personality,
		Capabilities:  t.Capabilities,
		Model:         t.Model,
		Temperature:   numericFromFloat32(t.Temperature),
		MaxTokens:     int32(t.MaxTokens),
		HandoffPolicy: t.HandoffPolicy,
		Metadata:      []byte(meta),
	})
	if err != nil {
		return nil, fmt.Errorf("upsert template: %w", err)
	}
	out := rowToTemplate(row.ID, row.Slug, row.Name, row.SystemPrompt, row.Personality,
		row.Capabilities, row.Model, row.Temperature, row.MaxTokens, row.HandoffPolicy, row.Metadata)
	return &out, nil
}

// GetBySlug devuelve un template específico.
func (s *TemplateStore) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*AgentTemplate, error) {
	row, err := s.q(ctx).GetAgentTemplateBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTemplateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get template: %w", err)
	}
	out := rowToTemplate(row.ID, row.Slug, row.Name, row.SystemPrompt, row.Personality,
		row.Capabilities, row.Model, row.Temperature, row.MaxTokens, row.HandoffPolicy, row.Metadata)
	return &out, nil
}

// List devuelve templates de la org.
func (s *TemplateStore) List(ctx context.Context, orgID uuid.UUID, limit int) ([]AgentTemplate, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.q(ctx).ListAgentTemplates(ctx, int32(limit))
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	out := make([]AgentTemplate, 0, len(rows))
	for _, row := range rows {
		out = append(out, rowToTemplate(row.ID, row.Slug, row.Name, row.SystemPrompt, row.Personality,
			row.Capabilities, row.Model, row.Temperature, row.MaxTokens, row.HandoffPolicy, row.Metadata))
	}
	return out, nil
}

// Delete remove template; usado por admin tools.
func (s *TemplateStore) Delete(ctx context.Context, orgID uuid.UUID, slug string) error {
	affected, err := s.q(ctx).DeleteAgentTemplate(ctx, slug)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	if affected == 0 {
		return ErrTemplateNotFound
	}
	return nil
}
