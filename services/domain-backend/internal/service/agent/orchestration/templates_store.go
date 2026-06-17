package orchestration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TemplateStore — issue-08.5 CRUD de agent_templates.
type TemplateStore struct {
	Pool *pgxpool.Pool
}

var ErrTemplateNotFound = errors.New("agent template not found")

// Upsert crea o actualiza por (org, slug). Devuelve la versión persistida.
func (s *TemplateStore) Upsert(ctx context.Context, orgID uuid.UUID, t AgentTemplate) (*AgentTemplate, error) {
	if t.HandoffPolicy == "" {
		t.HandoffPolicy = HandoffAllow
	}
	meta := t.Metadata
	if len(meta) == 0 {
		meta = json.RawMessage("{}")
	}
	var out AgentTemplate
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO agent_templates
		  (slug, name, system_prompt, personality, capabilities,
		   model, temperature, max_tokens, handoff_policy, metadata)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7, $8, $9, $10)
		ON CONFLICT (slug)
		DO UPDATE SET
		  name = EXCLUDED.name,
		  system_prompt = EXCLUDED.system_prompt,
		  personality = EXCLUDED.personality,
		  capabilities = EXCLUDED.capabilities,
		  model = EXCLUDED.model,
		  temperature = EXCLUDED.temperature,
		  max_tokens = EXCLUDED.max_tokens,
		  handoff_policy = EXCLUDED.handoff_policy,
		  metadata = EXCLUDED.metadata,
		  updated_at = now()
		RETURNING id, slug, name, system_prompt, COALESCE(personality, ''),
		          capabilities, model, temperature, max_tokens, handoff_policy, metadata`,
		t.Slug, t.Name, t.SystemPrompt, t.Personality, t.Capabilities,
		t.Model, t.Temperature, t.MaxTokens, t.HandoffPolicy, meta,
	).Scan(&out.ID, &out.Slug, &out.Name, &out.SystemPrompt, &out.Personality,
		&out.Capabilities, &out.Model, &out.Temperature, &out.MaxTokens,
		&out.HandoffPolicy, &out.Metadata)
	if err != nil {
		return nil, fmt.Errorf("upsert template: %w", err)
	}
	return &out, nil
}

// GetBySlug devuelve un template específico.
func (s *TemplateStore) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*AgentTemplate, error) {
	var t AgentTemplate
	err := s.Pool.QueryRow(ctx, `
		SELECT id, slug, name, system_prompt, COALESCE(personality, ''),
		       capabilities, model, temperature, max_tokens, handoff_policy, metadata
		FROM agent_templates WHERE slug = $1`,
		slug,
	).Scan(&t.ID, &t.Slug, &t.Name, &t.SystemPrompt, &t.Personality,
		&t.Capabilities, &t.Model, &t.Temperature, &t.MaxTokens,
		&t.HandoffPolicy, &t.Metadata)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTemplateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get template: %w", err)
	}
	return &t, nil
}

// List devuelve templates de la org.
func (s *TemplateStore) List(ctx context.Context, orgID uuid.UUID, limit int) ([]AgentTemplate, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, slug, name, system_prompt, COALESCE(personality, ''),
		       capabilities, model, temperature, max_tokens, handoff_policy, metadata
		FROM agent_templates
		ORDER BY slug ASC LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	defer rows.Close()
	var out []AgentTemplate
	for rows.Next() {
		var t AgentTemplate
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.SystemPrompt, &t.Personality,
			&t.Capabilities, &t.Model, &t.Temperature, &t.MaxTokens,
			&t.HandoffPolicy, &t.Metadata); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Delete remove template; usado por admin tools.
func (s *TemplateStore) Delete(ctx context.Context, orgID uuid.UUID, slug string) error {
	tag, err := s.Pool.Exec(ctx,
		`DELETE FROM agent_templates WHERE slug = $1`,
		slug,
	)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrTemplateNotFound
	}
	return nil
}
