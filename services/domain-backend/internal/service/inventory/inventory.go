// Package inventory — issue F3 inventario de capacidades al iniciar sesión.
// Devuelve el detalle completo de agents/skills/flows/mcp_providers/mcp_servers
// /project_templates/policies, combinando built-ins (organization_id NULL)
// y per-org del proyecto detectado.
//
// Diseñado para alimentar `domain detect` con info útil al agente IA:
// "qué tools/skills/agents tengo disponibles en este proyecto".

package inventory

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	Pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{Pool: pool}
}

type Inventory struct {
	Agents       []AgentSummary       `json:"agents"`
	Skills       []SkillSummary       `json:"skills"`
	Flows        []FlowSummary        `json:"flows"`
	MCPProviders []MCPProviderSummary `json:"mcp_providers"`
	MCPServers   []MCPServerSummary   `json:"mcp_servers"`
	Templates    []TemplateSummary    `json:"project_templates"`
	Policies     []PolicySummary      `json:"policies"`
}

type AgentSummary struct {
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Model        string   `json:"model"`
	Capabilities []string `json:"capabilities"`
}

type SkillSummary struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

type FlowSummary struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type MCPProviderSummary struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Command     string   `json:"command"`
	Tags        []string `json:"tags"`
	RequiredEnv []string `json:"required_env"`
}

type MCPServerSummary struct {
	Name      string `json:"name"`
	Transport string `json:"transport"`
	Status    string `json:"status"`
	Enabled   bool   `json:"enabled"`
}

type TemplateSummary struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type PolicySummary struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type LoadInput struct {
	ProjectID *string
	OrgID     *string
}

func (s *Service) Load(ctx context.Context, in LoadInput) (*Inventory, error) {
	inv := &Inventory{}

	if err := s.loadMCPProviders(ctx, inv); err != nil {
		return nil, fmt.Errorf("mcp_providers: %w", err)
	}
	if err := s.loadMCPServers(ctx, in.OrgID, inv); err != nil {
		return nil, fmt.Errorf("mcp_servers: %w", err)
	}
	if err := s.loadTemplates(ctx, inv); err != nil {
		return nil, fmt.Errorf("templates: %w", err)
	}
	if err := s.loadPolicies(ctx, inv); err != nil {
		return nil, fmt.Errorf("policies: %w", err)
	}
	if err := s.loadAgents(ctx, in.OrgID, inv); err != nil {
		return nil, fmt.Errorf("agents: %w", err)
	}
	if err := s.loadSkills(ctx, in.OrgID, inv); err != nil {
		return nil, fmt.Errorf("skills: %w", err)
	}
	if err := s.loadFlows(ctx, in.OrgID, inv); err != nil {
		return nil, fmt.Errorf("flows: %w", err)
	}
	return inv, nil
}

func (s *Service) loadMCPProviders(ctx context.Context, inv *Inventory) error {
	rows, err := s.Pool.Query(ctx, `
		SELECT name, description, command, tags, required_env
		FROM mcp_providers
		WHERE organization_id IS NULL
		ORDER BY name
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var p MCPProviderSummary
		if err := rows.Scan(&p.Name, &p.Description, &p.Command, &p.Tags, &p.RequiredEnv); err != nil {
			return err
		}
		inv.MCPProviders = append(inv.MCPProviders, p)
	}
	return nil
}

func (s *Service) loadMCPServers(ctx context.Context, orgID *string, inv *Inventory) error {
	q := `SELECT name, transport, status, enabled FROM mcp_servers WHERE enabled = TRUE`
	args := []any{}
	if orgID != nil {
		q += ` AND organization_id = $1`
		args = append(args, *orgID)
	}
	q += ` ORDER BY name`
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var m MCPServerSummary
		if err := rows.Scan(&m.Name, &m.Transport, &m.Status, &m.Enabled); err != nil {
			return err
		}
		inv.MCPServers = append(inv.MCPServers, m)
	}
	return nil
}

func (s *Service) loadTemplates(ctx context.Context, inv *Inventory) error {
	rows, err := s.Pool.Query(ctx, `
		SELECT slug, name, COALESCE(description, '')
		FROM project_templates
		WHERE organization_id IS NULL
		ORDER BY slug
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var t TemplateSummary
		if err := rows.Scan(&t.Slug, &t.Name, &t.Description); err != nil {
			return err
		}
		inv.Templates = append(inv.Templates, t)
	}
	return nil
}

func (s *Service) loadPolicies(ctx context.Context, inv *Inventory) error {
	rows, err := s.Pool.Query(ctx, `
		SELECT slug, name, LEFT(COALESCE(body_md, ''), 120)
		FROM platform_policies
		WHERE is_active = TRUE
		ORDER BY slug
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var p PolicySummary
		if err := rows.Scan(&p.Slug, &p.Name, &p.Description); err != nil {
			return err
		}
		inv.Policies = append(inv.Policies, p)
	}
	return nil
}

func (s *Service) loadAgents(ctx context.Context, orgID *string, inv *Inventory) error {
	if orgID == nil {
		return nil
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT slug, name, COALESCE(description, ''), COALESCE(model, ''),
		       COALESCE(skills_slugs, '{}')
		FROM agents
		WHERE organization_id = $1 AND deleted_at IS NULL
		ORDER BY slug
	`, *orgID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var a AgentSummary
		if err := rows.Scan(&a.Slug, &a.Name, &a.Description, &a.Model, &a.Capabilities); err != nil {
			return err
		}
		inv.Agents = append(inv.Agents, a)
	}
	return nil
}

func (s *Service) loadSkills(ctx context.Context, orgID *string, inv *Inventory) error {
	if orgID == nil {
		return nil
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT slug, name, COALESCE(description, ''), COALESCE(skill_type, 'prompt')
		FROM skills
		WHERE organization_id = $1 AND deleted_at IS NULL
		ORDER BY slug
	`, *orgID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sk SkillSummary
		if err := rows.Scan(&sk.Slug, &sk.Name, &sk.Description, &sk.Type); err != nil {
			return err
		}
		inv.Skills = append(inv.Skills, sk)
	}
	return nil
}

func (s *Service) loadFlows(ctx context.Context, orgID *string, inv *Inventory) error {
	if orgID == nil {
		return nil
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT slug, name, COALESCE(description, '')
		FROM flows
		WHERE organization_id = $1 AND deleted_at IS NULL
		ORDER BY slug
	`, *orgID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var f FlowSummary
		if err := rows.Scan(&f.Slug, &f.Name, &f.Description); err != nil {
			return err
		}
		inv.Flows = append(inv.Flows, f)
	}
	return nil
}
