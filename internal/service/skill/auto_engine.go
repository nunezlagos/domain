// HU-05.4 auto-skill-engine — descubre automáticamente nuevos skills desde
// tools MCP externos (HU-12.4) o desde código del repo (annotations).
//
// Cuando un MCP server externo registra una tool nueva, el auto-engine
// la materializa como un skill ejecutable en la BD. Idempotente: si la
// tool name ya existe como skill, actualiza schema en lugar de duplicar.
package skill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DiscoveredTool es la representación de una tool encontrada (vía MCP tools/
// list o annotation parser).
type DiscoveredTool struct {
	OrganizationID uuid.UUID       `json:"organization_id"`
	Source         string          `json:"source"`              // mcp_server:<id> | code_annotation
	SourceRef      string          `json:"source_ref,omitempty"`// path/file o mcp_server_id
	ToolName       string          `json:"tool_name"`
	Description    string          `json:"description,omitempty"`
	InputSchema    json.RawMessage `json:"input_schema,omitempty"`
	OutputSchema   json.RawMessage `json:"output_schema,omitempty"`
}

// MaterializeResult reporta qué se hizo per tool.
type MaterializeResult struct {
	ToolName   string `json:"tool_name"`
	SkillID    uuid.UUID `json:"skill_id"`
	Created    bool   `json:"created"` // true = nuevo skill; false = update
	Slug       string `json:"slug"`
}

// AutoEngine descubre y materializa skills desde sources externos.
type AutoEngine struct {
	Pool *pgxpool.Pool
}

// MaterializeAll procesa una lista de tools, creando/actualizando skills.
func (e *AutoEngine) MaterializeAll(ctx context.Context, tools []DiscoveredTool) ([]MaterializeResult, error) {
	results := make([]MaterializeResult, 0, len(tools))
	for _, t := range tools {
		res, err := e.Materialize(ctx, t)
		if err != nil {
			return results, fmt.Errorf("materialize %s: %w", t.ToolName, err)
		}
		results = append(results, *res)
	}
	return results, nil
}

// Materialize crea o actualiza un skill desde una tool descubierta.
func (e *AutoEngine) Materialize(ctx context.Context, t DiscoveredTool) (*MaterializeResult, error) {
	if strings.TrimSpace(t.ToolName) == "" {
		return nil, errors.New("tool_name required")
	}
	slug := sluggify(t.ToolName)

	tx, err := e.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var existing uuid.UUID
	err = tx.QueryRow(ctx,
		`SELECT id FROM skills WHERE organization_id = $1 AND slug = $2`,
		t.OrganizationID, slug,
	).Scan(&existing)

	if errors.Is(err, pgx.ErrNoRows) {
		// create
		var newID uuid.UUID
		err = tx.QueryRow(ctx, `
			INSERT INTO skills (organization_id, slug, name, description, input_schema, output_schema, source, source_ref)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id`,
			t.OrganizationID, slug, t.ToolName, t.Description,
			t.InputSchema, t.OutputSchema, t.Source, t.SourceRef,
		).Scan(&newID)
		if err != nil {
			return nil, fmt.Errorf("insert: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return &MaterializeResult{ToolName: t.ToolName, SkillID: newID, Created: true, Slug: slug}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lookup: %w", err)
	}

	// update — solo si schema cambió
	_, err = tx.Exec(ctx, `
		UPDATE skills
		SET description = COALESCE(NULLIF($1, ''), description),
		    input_schema = COALESCE($2, input_schema),
		    output_schema = COALESCE($3, output_schema),
		    updated_at = now()
		WHERE id = $4`,
		t.Description, t.InputSchema, t.OutputSchema, existing,
	)
	if err != nil {
		return nil, fmt.Errorf("update: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &MaterializeResult{ToolName: t.ToolName, SkillID: existing, Created: false, Slug: slug}, nil
}

func sluggify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		case r == '_' || r == '-' || r == ' ' || r == '.':
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
