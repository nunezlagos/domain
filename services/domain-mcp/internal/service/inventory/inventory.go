// Package inventory — catálogo de capacidades de la plataforma (agents, skills,
// flows, MCP providers/servers, templates y policies) para exponer al onboarding.
//
//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
package inventory

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/inventory/inventorydb"
	"nunezlagos/domain/internal/store/txctx"
)

type Service struct {
	Pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{Pool: pool}
}

// q retorna un *Queries sobre la tx del contexto (si el wireup la inyectó, para
// que RLS aplique) o sobre el pool crudo en su defecto.
func (s *Service) q(ctx context.Context) *inventorydb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return inventorydb.New(tx)
	}
	return inventorydb.New(s.Pool)
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
	rows, err := s.q(ctx).ListMCPProviders(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		inv.MCPProviders = append(inv.MCPProviders, toMCPProvider(r))
	}
	return nil
}

func (s *Service) loadMCPServers(ctx context.Context, orgID *string, inv *Inventory) error {
	rows, err := s.q(ctx).ListMCPServers(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		inv.MCPServers = append(inv.MCPServers, toMCPServer(r))
	}
	return nil
}

func (s *Service) loadTemplates(ctx context.Context, inv *Inventory) error {
	rows, err := s.q(ctx).ListProjectTemplates(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		inv.Templates = append(inv.Templates, toTemplate(r))
	}
	return nil
}

func (s *Service) loadPolicies(ctx context.Context, inv *Inventory) error {
	rows, err := s.q(ctx).ListPlatformPolicies(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		inv.Policies = append(inv.Policies, toPolicy(r))
	}
	return nil
}

func (s *Service) loadAgents(ctx context.Context, orgID *string, inv *Inventory) error {
	if orgID == nil {
		return nil
	}
	rows, err := s.q(ctx).ListAgents(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		inv.Agents = append(inv.Agents, toAgent(r))
	}
	return nil
}

func (s *Service) loadSkills(ctx context.Context, orgID *string, inv *Inventory) error {
	if orgID == nil {
		return nil
	}
	rows, err := s.q(ctx).ListSkills(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		inv.Skills = append(inv.Skills, toSkill(r))
	}
	return nil
}

func (s *Service) loadFlows(ctx context.Context, orgID *string, inv *Inventory) error {
	if orgID == nil {
		return nil
	}
	rows, err := s.q(ctx).ListFlows(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		inv.Flows = append(inv.Flows, toFlow(r))
	}
	return nil
}

func toMCPProvider(r inventorydb.ListMCPProvidersRow) MCPProviderSummary {
	return MCPProviderSummary{
		Name:        r.Name,
		Description: r.Description,
		Command:     r.Command,
		Tags:        r.Tags,
		RequiredEnv: r.RequiredEnv,
	}
}

func toMCPServer(r inventorydb.ListMCPServersRow) MCPServerSummary {
	return MCPServerSummary{
		Name:      r.Name,
		Transport: r.Transport,
		Status:    r.Status,
		Enabled:   r.Enabled,
	}
}

func toTemplate(r inventorydb.ListProjectTemplatesRow) TemplateSummary {
	return TemplateSummary{
		Slug:        r.Slug,
		Name:        r.Name,
		Description: r.Description,
	}
}

func toPolicy(r inventorydb.ListPlatformPoliciesRow) PolicySummary {
	return PolicySummary{
		Slug:        r.Slug,
		Name:        r.Name,
		Description: r.Description,
	}
}

func toAgent(r inventorydb.ListAgentsRow) AgentSummary {
	return AgentSummary{
		Slug:         r.Slug,
		Name:         r.Name,
		Description:  r.Description,
		Model:        r.Model,
		Capabilities: r.SkillsSlugs,
	}
}

func toSkill(r inventorydb.ListSkillsRow) SkillSummary {
	return SkillSummary{
		Slug:        r.Slug,
		Name:        r.Name,
		Description: r.Description,
		Type:        r.SkillType,
	}
}

func toFlow(r inventorydb.ListFlowsRow) FlowSummary {
	return FlowSummary{
		Slug:        r.Slug,
		Name:        r.Name,
		Description: r.Description,
	}
}
