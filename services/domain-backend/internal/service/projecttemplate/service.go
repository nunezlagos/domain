// Package projecttemplate — issue-01.4 CRUD de templates de proyecto.
//
// Los templates definen defaults (settings + skills + agents + flows slugs)
// que se asignan al crear un project con template_id.
package projecttemplate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUnknown    = errors.New("not_found")
	ErrInvalidSlug = errors.New("invalid_slug")
)

var reSlug = regexp.MustCompile(`^[a-z][a-z0-9-]{0,98}[a-z0-9]$|^[a-z]$`)

// Template definición declarativa.
type Template struct {
	ID             uuid.UUID       `json:"id"`
	OrganizationID *uuid.UUID      `json:"organization_id,omitempty"`
	Slug           string          `json:"slug"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	IsDefault      bool            `json:"is_default"`
	IsPublic       bool            `json:"is_public"`
	Settings       json.RawMessage `json:"settings"`
	DefaultSkills  []string        `json:"default_skills"`
	DefaultAgents  []string        `json:"default_agents"`
	DefaultFlows   []string        `json:"default_flows"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// CreateInput para POST.
type CreateInput struct {
	Slug          string
	Name          string
	Description   string
	IsDefault     bool
	IsPublic      bool
	Settings      map[string]any
	DefaultSkills []string
	DefaultAgents []string
	DefaultFlows  []string
}

type Service struct {
	Pool *pgxpool.Pool
}

func (s *Service) Create(ctx context.Context, orgID uuid.UUID, in CreateInput) (*Template, error) {
	if !reSlug.MatchString(in.Slug) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidSlug, in.Slug)
	}
	if in.Settings == nil {
		in.Settings = map[string]any{}
	}
	settingsJSON, _ := json.Marshal(in.Settings)
	row := s.Pool.QueryRow(ctx,
		`INSERT INTO project_templates
			(organization_id, slug, name, description, is_default, is_public,
			 settings, default_skills, default_agents, default_flows)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		 RETURNING id, created_at, updated_at`,
		orgID, in.Slug, in.Name, in.Description, in.IsDefault, in.IsPublic,
		settingsJSON, in.DefaultSkills, in.DefaultAgents, in.DefaultFlows)
	t := &Template{
		OrganizationID: &orgID, Slug: in.Slug, Name: in.Name,
		Description: in.Description, IsDefault: in.IsDefault, IsPublic: in.IsPublic,
		Settings: settingsJSON, DefaultSkills: in.DefaultSkills,
		DefaultAgents: in.DefaultAgents, DefaultFlows: in.DefaultFlows,
	}
	if err := row.Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, fmt.Errorf("insert: %w", err)
	}
	return t, nil
}

func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (*Template, error) {
	row := s.Pool.QueryRow(ctx,
		`SELECT id, organization_id, slug, name, COALESCE(description,''),
			is_default, is_public, settings, default_skills, default_agents,
			default_flows, created_at, updated_at
		 FROM project_templates
		 WHERE id=$1 AND (organization_id=$2 OR is_public=TRUE)`, id, orgID)
	var t Template
	err := row.Scan(&t.ID, &t.OrganizationID, &t.Slug, &t.Name, &t.Description,
		&t.IsDefault, &t.IsPublic, &t.Settings, &t.DefaultSkills,
		&t.DefaultAgents, &t.DefaultFlows, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUnknown
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Service) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Template, error) {
	row := s.Pool.QueryRow(ctx,
		`SELECT id, organization_id, slug, name, COALESCE(description,''),
			is_default, is_public, settings, default_skills, default_agents,
			default_flows, created_at, updated_at
		 FROM project_templates
		 WHERE slug=$1 AND (organization_id=$2 OR is_public=TRUE)`, slug, orgID)
	var t Template
	err := row.Scan(&t.ID, &t.OrganizationID, &t.Slug, &t.Name, &t.Description,
		&t.IsDefault, &t.IsPublic, &t.Settings, &t.DefaultSkills,
		&t.DefaultAgents, &t.DefaultFlows, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUnknown
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Service) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]Template, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, organization_id, slug, name, COALESCE(description,''),
			is_default, is_public, settings, default_skills, default_agents,
			default_flows, created_at, updated_at
		 FROM project_templates
		 WHERE organization_id=$1 OR is_public=TRUE
		 ORDER BY is_default DESC, name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Template
	for rows.Next() {
		var t Template
		if err := rows.Scan(&t.ID, &t.OrganizationID, &t.Slug, &t.Name, &t.Description,
			&t.IsDefault, &t.IsPublic, &t.Settings, &t.DefaultSkills,
			&t.DefaultAgents, &t.DefaultFlows, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	ct, err := s.Pool.Exec(ctx,
		`DELETE FROM project_templates WHERE id=$1 AND organization_id=$2`, id, orgID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrUnknown
	}
	return nil
}
