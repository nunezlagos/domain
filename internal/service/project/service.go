// Package project — CRUD básico de projects scoped por organization.
//
// Project es el bucket donde viven observations, prompts, knowledge_docs.
// Slug único por (organization_id, slug). Soft-delete con deleted_at.
package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/saargo/domain/internal/audit"
)

var (
	ErrSlugInvalid = errors.New("slug must be lowercase ascii, digits, dashes; 2-100 chars")
	ErrSlugTaken   = errors.New("project slug already taken in this organization")
	ErrNotFound    = errors.New("project not found")
)

var reSlug = regexp.MustCompile(`^[a-z][a-z0-9-]{0,98}[a-z0-9]$|^[a-z]$`)

type Project struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Slug           string
	Description    string
	RepositoryURL  string
	TemplateID     *uuid.UUID
	Settings       map[string]any
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

type CreateInput struct {
	OrganizationID uuid.UUID
	Name           string
	Slug           string
	Description    string
	RepositoryURL  string
	TemplateID     *uuid.UUID
	Settings       map[string]any
	ActorID        uuid.UUID
}

type Service struct {
	Pool  *pgxpool.Pool
	Audit audit.Recorder
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Project, error) {
	in.Slug = strings.TrimSpace(in.Slug)
	if !reSlug.MatchString(in.Slug) {
		return nil, ErrSlugInvalid
	}
	if in.Settings == nil {
		in.Settings = map[string]any{}
	}
	settingsJSON, _ := json.Marshal(in.Settings)

	var p Project
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO projects (organization_id, name, slug, description, repository_url, template_id, settings)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, organization_id, name, slug, COALESCE(description,''),
		           COALESCE(repository_url,''), template_id, settings, created_at, updated_at, deleted_at`,
		in.OrganizationID, in.Name, in.Slug, nullStr(in.Description), nullStr(in.RepositoryURL),
		in.TemplateID, settingsJSON,
	).Scan(&p.ID, &p.OrganizationID, &p.Name, &p.Slug, &p.Description,
		&p.RepositoryURL, &p.TemplateID, &p.Settings, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt)
	if err != nil {
		if strings.Contains(err.Error(), "projects_organization_id_slug_key") ||
			strings.Contains(err.Error(), "duplicate key") {
			return nil, ErrSlugTaken
		}
		return nil, fmt.Errorf("insert project: %w", err)
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &in.OrganizationID,
			ActorID:        &in.ActorID,
			ActorType:      audit.ActorUser,
			Action:         "project.created",
			EntityType:     "project",
			EntityID:       &p.ID,
			NewValues:      map[string]any{"name": p.Name, "slug": p.Slug},
		})
	}
	return &p, nil
}

func (s *Service) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Project, error) {
	return s.queryOne(ctx,
		`WHERE organization_id = $1 AND slug = $2 AND deleted_at IS NULL`, orgID, slug)
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Project, error) {
	return s.queryOne(ctx, `WHERE id = $1`, id)
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]Project, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, organization_id, name, slug, COALESCE(description,''),
		        COALESCE(repository_url,''), template_id, settings, created_at, updated_at, deleted_at
		 FROM projects
		 WHERE organization_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.OrganizationID, &p.Name, &p.Slug, &p.Description,
			&p.RepositoryURL, &p.TemplateID, &p.Settings, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type UpdateInput struct {
	Name          *string
	Description   *string
	RepositoryURL *string
	Settings      map[string]any
	ActorID       uuid.UUID
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Project, error) {
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
	repo := prev.RepositoryURL
	if in.RepositoryURL != nil {
		repo = *in.RepositoryURL
	}
	settings := prev.Settings
	if in.Settings != nil {
		settings = in.Settings
	}
	settingsJSON, _ := json.Marshal(settings)

	var p Project
	err = s.Pool.QueryRow(ctx,
		`UPDATE projects
		 SET name = $2, description = $3, repository_url = $4, settings = $5
		 WHERE id = $1 AND deleted_at IS NULL
		 RETURNING id, organization_id, name, slug, COALESCE(description,''),
		           COALESCE(repository_url,''), template_id, settings, created_at, updated_at, deleted_at`,
		id, name, nullStr(desc), nullStr(repo), settingsJSON,
	).Scan(&p.ID, &p.OrganizationID, &p.Name, &p.Slug, &p.Description,
		&p.RepositoryURL, &p.TemplateID, &p.Settings, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update project: %w", err)
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &p.OrganizationID,
			ActorID:        &in.ActorID,
			ActorType:      audit.ActorUser,
			Action:         "project.updated",
			EntityType:     "project",
			EntityID:       &p.ID,
		})
	}
	return &p, nil
}

func (s *Service) SoftDelete(ctx context.Context, id, actorID uuid.UUID) error {
	prev, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if prev.DeletedAt != nil {
		return nil
	}
	_, err = s.Pool.Exec(ctx,
		`UPDATE projects SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &prev.OrganizationID,
			ActorID:        &actorID,
			ActorType:      audit.ActorUser,
			Action:         "project.deleted",
			EntityType:     "project",
			EntityID:       &id,
		})
	}
	return nil
}

func (s *Service) queryOne(ctx context.Context, where string, args ...any) (*Project, error) {
	var p Project
	q := `SELECT id, organization_id, name, slug, COALESCE(description,''),
	        COALESCE(repository_url,''), template_id, settings, created_at, updated_at, deleted_at
	      FROM projects ` + where
	err := s.Pool.QueryRow(ctx, q, args...).Scan(
		&p.ID, &p.OrganizationID, &p.Name, &p.Slug, &p.Description,
		&p.RepositoryURL, &p.TemplateID, &p.Settings, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query project: %w", err)
	}
	return &p, nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
