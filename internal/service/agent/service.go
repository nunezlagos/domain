// Package agent — issue-08.1 agent definitions CRUD.
//
// Un agent compone:
//   - model + provider (claude-sonnet-4-6 / claude-opus-4-7 / etc.)
//   - system_prompt (puede referenciar prompt templates por slug)
//   - skills_slugs []string (la lista de skills que tiene acceso a ejecutar)
//   - guardrails: max_iterations, token_budget, temperature
//
// La ejecución (run) vive en issue-08.2, separada. Aquí solo CRUD + validación.
package agent

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
)

var (
	ErrSlugInvalid      = errors.New("slug must be lowercase ascii, digits, dashes (2-100 chars)")
	ErrSlugTaken        = errors.New("slug already taken in this organization")
	ErrNameRequired     = errors.New("name required")
	ErrModelRequired    = errors.New("model required")
	ErrProviderInvalid  = errors.New("provider must be one of: anthropic, openai, google, ollama")
	ErrSkillNotFound    = errors.New("one or more skills_slugs do not exist in this organization")
	ErrNotFound         = errors.New("agent not found")
	ErrTemperatureRange = errors.New("temperature must be within [0, 2]")
	ErrModelUnknown     = errors.New("model not found in model_registry for this provider")
)

// maxVersionsKept límite de snapshots en agent_versions por agent.
const maxVersionsKept = 50

var (
	reSlug      = regexp.MustCompile(`^[a-z][a-z0-9-]{0,98}[a-z0-9]$|^[a-z]$`)
	validProviders = map[string]bool{
		"anthropic": true, "openai": true, "google": true, "ollama": true,
	}
)

type Agent struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	Slug            string
	Name            string
	Description     string
	Provider        string
	Model           string
	SystemPrompt    string
	SkillsSlugs     []string
	MaxIterations   int
	TokenBudget     *int64
	Temperature     *float64
	SeedManaged     bool
	SeedVersion     *int
	IsUserModified  bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CreateInput struct {
	OrganizationID uuid.UUID
	Slug           string
	Name           string
	Description    string
	Provider       string
	Model          string
	SystemPrompt   string
	SkillsSlugs    []string
	MaxIterations  int
	TokenBudget    *int64
	Temperature    *float64
	ActorID        uuid.UUID
}

type UpdateInput struct {
	Name          *string
	Description   *string
	Provider      *string
	Model         *string
	SystemPrompt  *string
	SkillsSlugs   []string
	MaxIterations *int
	TokenBudget   *int64
	Temperature   *float64
	ActorID       uuid.UUID
}

type Service struct {
	Pool  *pgxpool.Pool
	Audit audit.Recorder
}

// validateSkills verifica que todos los slugs existan en la org como skills
// activos. Defense in depth: aplicación valida + Eventual constraint en BD
// si se agregara FK.
func (s *Service) validateSkills(ctx context.Context, orgID uuid.UUID, slugs []string) error {
	if len(slugs) == 0 {
		return nil
	}
	var foundCount int
	err := s.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM skills
		 WHERE organization_id = $1 AND slug = ANY($2) AND deleted_at IS NULL`,
		orgID, slugs,
	).Scan(&foundCount)
	if err != nil {
		return fmt.Errorf("validate skills: %w", err)
	}
	if foundCount != len(slugs) {
		return ErrSkillNotFound
	}
	return nil
}

// validateTemperature rechaza valores fuera de [0, 2].
func validateTemperature(t *float64) error {
	if t != nil && (*t < 0 || *t > 2) {
		return ErrTemperatureRange
	}
	return nil
}

// validateModel verifica que el modelo exista activo en model_registry.
// ollama se exime: permite modelos locales arbitrarios (auto-pull issue-06.3).
func (s *Service) validateModel(ctx context.Context, provider, model string) error {
	if provider == "ollama" {
		return nil
	}
	var exists bool
	err := s.Pool.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM model_registry
		   WHERE provider = $1 AND model = $2 AND modality = 'completion' AND is_active = TRUE)`,
		provider, model).Scan(&exists)
	if err != nil {
		return fmt.Errorf("validate model: %w", err)
	}
	if !exists {
		return ErrModelUnknown
	}
	return nil
}

// generateSlug deriva un slug desde el name y resuelve colisiones con -2..-N.
func (s *Service) generateSlug(ctx context.Context, orgID uuid.UUID, name string) (string, error) {
	base := slugify(name)
	if base == "" {
		return "", ErrSlugInvalid
	}
	candidate := base
	for i := 2; i <= 50; i++ {
		var taken bool
		err := s.Pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM agents
			 WHERE organization_id = $1 AND slug = $2 AND deleted_at IS NULL)`,
			orgID, candidate).Scan(&taken)
		if err != nil {
			return "", fmt.Errorf("check slug: %w", err)
		}
		if !taken {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
	return "", ErrSlugTaken
}

func slugify(name string) string {
	var b strings.Builder
	prevDash := true // evita dash inicial
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case !prevDash:
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 100 {
		out = strings.Trim(out[:100], "-")
	}
	return out
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Agent, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, ErrNameRequired
	}
	if in.Slug == "" {
		slug, err := s.generateSlug(ctx, in.OrganizationID, in.Name)
		if err != nil {
			return nil, err
		}
		in.Slug = slug
	}
	if !reSlug.MatchString(in.Slug) {
		return nil, ErrSlugInvalid
	}
	if strings.TrimSpace(in.Model) == "" {
		return nil, ErrModelRequired
	}
	if !validProviders[in.Provider] {
		return nil, ErrProviderInvalid
	}
	if err := s.validateModel(ctx, in.Provider, in.Model); err != nil {
		return nil, err
	}
	if err := validateTemperature(in.Temperature); err != nil {
		return nil, err
	}
	if in.SkillsSlugs == nil {
		in.SkillsSlugs = []string{}
	}
	if err := s.validateSkills(ctx, in.OrganizationID, in.SkillsSlugs); err != nil {
		return nil, err
	}
	if in.MaxIterations <= 0 {
		in.MaxIterations = 20
	}

	var a Agent
	err := s.Pool.QueryRow(ctx,
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
		if strings.Contains(err.Error(), "agents_organization_id_slug_key") ||
			strings.Contains(err.Error(), "duplicate key") {
			return nil, ErrSlugTaken
		}
		return nil, fmt.Errorf("insert agent: %w", err)
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &in.OrganizationID,
			ActorID:        &in.ActorID,
			ActorType:      audit.ActorUser,
			Action:         "agent.created",
			EntityType:     "agent",
			EntityID:       &a.ID,
			NewValues: map[string]any{
				"slug": a.Slug, "model": a.Model, "skills_count": len(a.SkillsSlugs),
			},
		})
	}
	return &a, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Agent, error) {
	prev, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	name := prev.Name
	if in.Name != nil {
		name = *in.Name
	}
	desc := prev.Description
	if in.Description != nil {
		desc = *in.Description
	}
	provider := prev.Provider
	if in.Provider != nil {
		if !validProviders[*in.Provider] {
			return nil, ErrProviderInvalid
		}
		provider = *in.Provider
	}
	model := prev.Model
	if in.Model != nil {
		model = *in.Model
	}
	if in.Model != nil || in.Provider != nil {
		if err := s.validateModel(ctx, provider, model); err != nil {
			return nil, err
		}
	}
	if err := validateTemperature(in.Temperature); err != nil {
		return nil, err
	}
	sp := prev.SystemPrompt
	if in.SystemPrompt != nil {
		sp = *in.SystemPrompt
	}
	skills := prev.SkillsSlugs
	if in.SkillsSlugs != nil {
		if err := s.validateSkills(ctx, prev.OrganizationID, in.SkillsSlugs); err != nil {
			return nil, err
		}
		skills = in.SkillsSlugs
	}
	maxIter := prev.MaxIterations
	if in.MaxIterations != nil {
		maxIter = *in.MaxIterations
	}
	tokenBudget := prev.TokenBudget
	if in.TokenBudget != nil {
		tokenBudget = in.TokenBudget
	}
	temp := prev.Temperature
	if in.Temperature != nil {
		temp = in.Temperature
	}
	userMod := prev.IsUserModified || prev.SeedManaged

	var a Agent
	err = s.Pool.QueryRow(ctx,
		`UPDATE agents
		 SET name = $2, description = $3, provider = $4, model = $5,
		     system_prompt = $6, skills_slugs = $7, max_iterations = $8,
		     token_budget = $9, temperature = $10, is_user_modified = $11
		 WHERE id = $1 AND deleted_at IS NULL
		 RETURNING id, organization_id, slug, name, COALESCE(description,''),
		           provider, model, COALESCE(system_prompt,''), skills_slugs,
		           max_iterations, token_budget, temperature,
		           seed_managed, seed_version, is_user_modified, created_at, updated_at`,
		id, name, nullStr(desc), provider, model, nullStr(sp), skills,
		maxIter, tokenBudget, temp, userMod,
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
	if err := s.archiveVersion(ctx, prev, in.ActorID); err != nil {
		return nil, err
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &a.OrganizationID,
			ActorID:        &in.ActorID,
			ActorType:      audit.ActorUser,
			Action:         "agent.updated",
			EntityType:     "agent",
			EntityID:       &a.ID,
		})
	}
	return &a, nil
}

// archiveVersion guarda snapshot de la config previa en agent_versions y
// purga versiones por encima de maxVersionsKept.
func (s *Service) archiveVersion(ctx context.Context, prev *Agent, actorID uuid.UUID) error {
	snapshot := map[string]any{
		"name": prev.Name, "description": prev.Description,
		"provider": prev.Provider, "model": prev.Model,
		"system_prompt": prev.SystemPrompt, "skills_slugs": prev.SkillsSlugs,
		"max_iterations": prev.MaxIterations, "token_budget": prev.TokenBudget,
		"temperature": prev.Temperature,
	}
	var changedBy any
	if actorID != uuid.Nil {
		changedBy = actorID
	}
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO agent_versions (agent_id, version, snapshot, changed_by)
		 SELECT $1, COALESCE(MAX(version), 0) + 1, $2, $3
		 FROM agent_versions WHERE agent_id = $1`,
		prev.ID, snapshot, changedBy)
	if err != nil {
		return fmt.Errorf("archive agent version: %w", err)
	}
	_, err = s.Pool.Exec(ctx,
		`DELETE FROM agent_versions
		 WHERE agent_id = $1 AND version <= (
		   SELECT MAX(version) - $2 FROM agent_versions WHERE agent_id = $1)`,
		prev.ID, maxVersionsKept)
	if err != nil {
		return fmt.Errorf("purge agent versions: %w", err)
	}
	return nil
}

// AgentVersion entrada del historial.
type AgentVersion struct {
	Version   int            `json:"version"`
	Snapshot  map[string]any `json:"snapshot"`
	ChangedBy *uuid.UUID     `json:"changed_by,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// GetVersions historial de un agent, más reciente primero.
func (s *Service) GetVersions(ctx context.Context, id uuid.UUID, limit int) ([]AgentVersion, error) {
	if _, err := s.GetByID(ctx, id); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx,
		`SELECT version, snapshot, changed_by, created_at
		 FROM agent_versions WHERE agent_id = $1
		 ORDER BY version DESC LIMIT $2`, id, limit)
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

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Agent, error) {
	return s.queryOne(ctx, `WHERE id = $1 AND deleted_at IS NULL`, id)
}

func (s *Service) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Agent, error) {
	return s.queryOne(ctx,
		`WHERE organization_id = $1 AND slug = $2 AND deleted_at IS NULL`, orgID, slug)
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID, limit int) ([]Agent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx,
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

func (s *Service) SoftDelete(ctx context.Context, id, actorID uuid.UUID) error {
	prev, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx,
		`UPDATE agents SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &prev.OrganizationID,
			ActorID:        &actorID,
			ActorType:      audit.ActorUser,
			Action:         "agent.deleted",
			EntityType:     "agent",
			EntityID:       &id,
		})
	}
	return nil
}

func (s *Service) queryOne(ctx context.Context, where string, args ...any) (*Agent, error) {
	var a Agent
	q := `SELECT id, organization_id, slug, name, COALESCE(description,''),
	        provider, model, COALESCE(system_prompt,''), skills_slugs,
	        max_iterations, token_budget, temperature,
	        seed_managed, seed_version, is_user_modified, created_at, updated_at
	      FROM agents ` + where
	err := s.Pool.QueryRow(ctx, q, args...).Scan(
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

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
