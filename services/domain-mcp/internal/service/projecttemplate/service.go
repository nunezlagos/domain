// Package projecttemplate — issue-01.4 CRUD de templates de proyecto.
//
// Los templates definen defaults (settings + skills + agents + flows slugs)
// que se asignan al crear un project con template_id.
//
//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
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

	"nunezlagos/domain/internal/service/projecttemplate/projecttemplatedb"
)

var (
	ErrUnknown     = errors.New("not_found")
	ErrInvalidSlug = errors.New("invalid_slug")
)

var reSlug = regexp.MustCompile(`^[a-z][a-z0-9-]{0,98}[a-z0-9]$|^[a-z]$`)

// Template definición declarativa.
type Template struct {
	ID            uuid.UUID       `json:"id"`
	Slug          string          `json:"slug"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	IsDefault     bool            `json:"is_default"`
	IsPublic      bool            `json:"is_public"`
	Settings      json.RawMessage `json:"settings"`
	DefaultSkills []string        `json:"default_skills"`
	DefaultAgents []string        `json:"default_agents"`
	DefaultFlows  []string        `json:"default_flows"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
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

func (s *Service) q() *projecttemplatedb.Queries { return projecttemplatedb.New(s.Pool) }

func (s *Service) Create(ctx context.Context, orgID uuid.UUID, in CreateInput) (*Template, error) {
	if !reSlug.MatchString(in.Slug) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidSlug, in.Slug)
	}
	if in.Settings == nil {
		in.Settings = map[string]any{}
	}
	settingsJSON, _ := json.Marshal(in.Settings)

	var desc *string
	if in.Description != "" {
		desc = &in.Description
	}

	row, err := s.q().InsertTemplate(ctx, projecttemplatedb.InsertTemplateParams{
		Slug:          in.Slug,
		Name:          in.Name,
		Description:   desc,
		IsDefault:     in.IsDefault,
		IsPublic:      in.IsPublic,
		Settings:      settingsJSON,
		DefaultSkills: in.DefaultSkills,
		DefaultAgents: in.DefaultAgents,
		DefaultFlows:  in.DefaultFlows,
	})
	if err != nil {
		return nil, fmt.Errorf("insert: %w", err)
	}

	return &Template{
		ID:            row.ID,
		Slug:          in.Slug,
		Name:          in.Name,
		Description:   in.Description,
		IsDefault:     in.IsDefault,
		IsPublic:      in.IsPublic,
		Settings:      settingsJSON,
		DefaultSkills: in.DefaultSkills,
		DefaultAgents: in.DefaultAgents,
		DefaultFlows:  in.DefaultFlows,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}, nil
}

func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (*Template, error) {
	row, err := s.q().GetTemplateByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUnknown
	}
	if err != nil {
		return nil, err
	}
	t := toTemplate(projecttemplatedb.ListTemplatesRow(row))
	return &t, nil
}

func (s *Service) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Template, error) {
	row, err := s.q().GetTemplateBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUnknown
	}
	if err != nil {
		return nil, err
	}
	t := toTemplate(projecttemplatedb.ListTemplatesRow(row))
	return &t, nil
}

func (s *Service) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]Template, error) {
	rows, err := s.q().ListTemplates(ctx)
	if err != nil {
		return nil, err
	}
	var out []Template
	for _, row := range rows {
		out = append(out, toTemplate(row))
	}
	return out, nil
}

func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	n, err := s.q().DeleteTemplate(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrUnknown
	}
	return nil
}

func toTemplate(r projecttemplatedb.ListTemplatesRow) Template {
	return Template{
		ID:            r.ID,
		Slug:          r.Slug,
		Name:          r.Name,
		Description:   r.Description,
		IsDefault:     r.IsDefault,
		IsPublic:      r.IsPublic,
		Settings:      r.Settings,
		DefaultSkills: r.DefaultSkills,
		DefaultAgents: r.DefaultAgents,
		DefaultFlows:  r.DefaultFlows,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
}
