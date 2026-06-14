// Package project — CRUD básico de projects scoped por organization.
//
// Project es el bucket donde viven observations, prompts, knowledge_docs.
// Slug único por (organization_id, slug). Soft-delete con deleted_at.
//
// HU-28.1: Service depende de Repository (interfaz). Pool sigue siendo
// público como deprecated para Strangler Fig.
package project

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/service/projecttemplate"
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
	// Pool — DEPRECATED (HU-28.1). Strangler Fig: callers que construyen
	// &Service{Pool: ...} siguen funcionando.
	Pool         *pgxpool.Pool
	Audit        audit.Recorder
	TemplateSvc  *projecttemplate.Service // opcional — issue-01.4 apply template on create

	repo Repository
}

// NewService construye el Service con dependencias explícitas.
func NewService(pool *pgxpool.Pool, audit audit.Recorder, tplSvc *projecttemplate.Service, repo Repository) *Service {
	if repo == nil && pool != nil {
		repo = NewPgRepository(pool)
	}
	return &Service{Pool: pool, Audit: audit, TemplateSvc: tplSvc, repo: repo}
}

func (s *Service) repository() Repository {
	if s.repo != nil {
		return s.repo
	}
	s.repo = NewPgRepository(s.Pool)
	return s.repo
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Project, error) {
	in.Slug = strings.TrimSpace(in.Slug)
	if !reSlug.MatchString(in.Slug) {
		return nil, ErrSlugInvalid
	}
	// issue-01.4: si se pasa template_id, mergear settings del template con override del request.
	if in.TemplateID != nil && s.TemplateSvc != nil {
		tpl, err := s.TemplateSvc.Get(ctx, in.OrganizationID, *in.TemplateID)
		if err == nil && tpl != nil {
			tplSettings := map[string]any{}
			if len(tpl.Settings) > 0 {
				_ = json.Unmarshal(tpl.Settings, &tplSettings)
			}
			// Template settings como base, request settings como override
			merged := make(map[string]any, len(tplSettings)+len(in.Settings))
			for k, v := range tplSettings {
				merged[k] = v
			}
			for k, v := range in.Settings {
				merged[k] = v
			}
			in.Settings = merged
		}
	}
	if in.Settings == nil {
		in.Settings = map[string]any{}
	}
	settingsJSON, _ := json.Marshal(in.Settings)

	p, err := s.repository().Insert(ctx, InsertParams{
		OrganizationID: in.OrganizationID,
		Name:           in.Name,
		Slug:           in.Slug,
		Description:    in.Description,
		RepositoryURL:  in.RepositoryURL,
		TemplateID:     in.TemplateID,
		SettingsJSON:   settingsJSON,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return nil, ErrSlugTaken
		}
		return nil, err
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			OrganizationID: &in.OrganizationID,
			ActorID:        &in.ActorID,
			ActorType:      audit.ActorUser,
			Action:         "project.created",
			EntityType:     "project",
			EntityID:       &p.ID,
			NewValues:      map[string]any{"name": p.Name, "slug": p.Slug},
		})
	}
	return p, nil
}

func (s *Service) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Project, error) {
	return s.repository().GetBySlug(ctx, orgID, slug)
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Project, error) {
	return s.repository().GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]Project, error) {
	return s.repository().List(ctx, orgID)
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

	p, err := s.repository().Update(ctx, id, UpdateParams{
		Name:          name,
		Description:   desc,
		RepositoryURL: repo,
		SettingsJSON:  settingsJSON,
	})
	if err != nil {
		return nil, err
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			OrganizationID: &p.OrganizationID,
			ActorID:        &in.ActorID,
			ActorType:      audit.ActorUser,
			Action:         "project.updated",
			EntityType:     "project",
			EntityID:       &p.ID,
		})
	}
	return p, nil
}

func (s *Service) SoftDelete(ctx context.Context, id, actorID uuid.UUID) error {
	prev, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if prev.DeletedAt != nil {
		return nil
	}
	if err := s.repository().SoftDelete(ctx, id); err != nil {
		return err
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
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

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
