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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	clientsvc "nunezlagos/domain/internal/service/client"
	"nunezlagos/domain/internal/service/project/projectdb"
	"nunezlagos/domain/internal/service/projecttemplate"
	"nunezlagos/domain/internal/store/txctx"
)

var (
	ErrSlugInvalid    = errors.New("slug must be lowercase ascii, digits, dashes; 2-100 chars")
	ErrSlugTaken      = errors.New("project slug already taken in this organization")
	ErrNotFound       = errors.New("project not found")
	ErrClientNotFound = errors.New("client_slug references a client that does not exist in this organization")
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

	ClientID *uuid.UUID

	ClientSlug string
	ClientName string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time
}

type CreateInput struct {
	OrganizationID uuid.UUID
	Name           string
	Slug           string
	Description    string
	RepositoryURL  string
	TemplateID     *uuid.UUID
	Settings       map[string]any

	ClientSlug string
	ActorID    uuid.UUID
}

type Service struct {
	Pool        *pgxpool.Pool
	Audit       audit.Recorder
	TemplateSvc *projecttemplate.Service // opcional — issue-01.4 apply template on create

	ClientSvc *clientsvc.Service
}

// NewService construye el Service con dependencias explícitas.
// El parámetro repo se mantiene por compatibilidad con callers existentes pero ya no se usa.
func NewService(pool *pgxpool.Pool, audit audit.Recorder, tplSvc *projecttemplate.Service, _ interface{}) *Service {
	return &Service{Pool: pool, Audit: audit, TemplateSvc: tplSvc}
}

// WithClientService inyecta el ClientService para resolver client_slug.
// Fluent setter para no romper firmas de NewService existentes (Strangler Fig).
func (s *Service) WithClientService(cs *clientsvc.Service) *Service {
	s.ClientSvc = cs
	return s
}

func (s *Service) q(ctx context.Context) *projectdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return projectdb.New(tx)
	}
	return projectdb.New(s.Pool)
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Project, error) {
	in.Slug = strings.TrimSpace(in.Slug)
	if !reSlug.MatchString(in.Slug) {
		return nil, ErrSlugInvalid
	}

	if in.TemplateID != nil && s.TemplateSvc != nil {
		tpl, err := s.TemplateSvc.Get(ctx, in.OrganizationID, *in.TemplateID)
		if err == nil && tpl != nil {
			tplSettings := map[string]any{}
			if len(tpl.Settings) > 0 {
				_ = json.Unmarshal(tpl.Settings, &tplSettings)
			}

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

	var clientID *uuid.UUID
	if strings.TrimSpace(in.ClientSlug) != "" {
		if s.ClientSvc == nil {
			return nil, errors.New("client_slug provided but ClientService not configured")
		}
		c, err := s.ClientSvc.Get(ctx, in.OrganizationID, in.ClientSlug)
		if err != nil {
			if errors.Is(err, clientsvc.ErrClientNotFound) {
				return nil, ErrClientNotFound
			}
			return nil, err
		}
		clientID = &c.ID
	}

	var desc *string
	if in.Description != "" {
		desc = &in.Description
	}
	var repoURL *string
	if in.RepositoryURL != "" {
		repoURL = &in.RepositoryURL
	}

	id, err := s.q(ctx).InsertProject(ctx, projectdb.InsertProjectParams{
		Name:          in.Name,
		Slug:          in.Slug,
		Description:   desc,
		RepositoryUrl: repoURL,
		TemplateID:    in.TemplateID,
		Settings:      settingsJSON,
		ClientID:      clientID,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return nil, ErrSlugTaken
		}
		return nil, err
	}

	p, err := s.GetByID(ctx, id)
	if err != nil {
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
	_ = orgID
	row, err := s.q(ctx).GetProjectBySlug(ctx, slug)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return rowToProject(row.ID, row.Name, row.Slug, row.Description, row.RepositoryUrl,
		row.TemplateID, row.Settings, row.ClientID,
		row.ClientSlug, row.ClientName,
		row.CreatedAt, row.UpdatedAt, row.DeletedAt), nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Project, error) {
	row, err := s.q(ctx).GetProjectByID(ctx, id)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return rowToProject(row.ID, row.Name, row.Slug, row.Description, row.RepositoryUrl,
		row.TemplateID, row.Settings, row.ClientID,
		row.ClientSlug, row.ClientName,
		row.CreatedAt, row.UpdatedAt, row.DeletedAt), nil
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]Project, error) {
	_ = orgID
	return s.listByFilter(ctx, nil)
}

// ListFiltered (REQ-28.2): variante con filtros (client_slug por ahora).
// Mantiene List() retrocompatible para los callers existentes (MCP, etc).
func (s *Service) ListFiltered(ctx context.Context, orgID uuid.UUID, clientSlug string) ([]Project, error) {
	var clientID *uuid.UUID
	clientSlug = strings.TrimSpace(clientSlug)
	if clientSlug != "" {
		if s.ClientSvc == nil {
			return nil, errors.New("client_slug filter provided but ClientService not configured")
		}
		c, err := s.ClientSvc.Get(ctx, orgID, clientSlug)
		if err != nil {
			if errors.Is(err, clientsvc.ErrClientNotFound) {
				return nil, ErrClientNotFound
			}
			return nil, err
		}
		id := c.ID
		clientID = &id
	}
	return s.listByFilter(ctx, clientID)
}

func (s *Service) listByFilter(ctx context.Context, clientID *uuid.UUID) ([]Project, error) {
	rows, err := s.q(ctx).ListProjects(ctx, clientID)
	if err != nil {
		return nil, err
	}
	out := make([]Project, 0, len(rows))
	for _, r := range rows {
		out = append(out, *rowToProject(r.ID, r.Name, r.Slug, r.Description, r.RepositoryUrl,
			r.TemplateID, r.Settings, r.ClientID,
			r.ClientSlug, r.ClientName,
			r.CreatedAt, r.UpdatedAt, r.DeletedAt))
	}
	return out, nil
}

type UpdateInput struct {
	Name          *string
	Description   *string
	RepositoryURL *string
	Settings      map[string]any

	ClientSlug *string
	ActorID    uuid.UUID
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

	var descPtr *string
	if desc != "" {
		descPtr = &desc
	}
	var repoPtr *string
	if repo != "" {
		repoPtr = &repo
	}

	clientID := prev.ClientID
	clientChanged := false
	if in.ClientSlug != nil {
		clientChanged = true
		raw := strings.TrimSpace(*in.ClientSlug)
		if raw == "" {
			clientID = nil // unset explícito
		} else {
			if s.ClientSvc == nil {
				return nil, errors.New("client_slug provided but ClientService not configured")
			}
			c, err := s.ClientSvc.Get(ctx, prev.OrganizationID, raw)
			if err != nil {
				if errors.Is(err, clientsvc.ErrClientNotFound) {
					return nil, ErrClientNotFound
				}
				return nil, err
			}
			clientID = &c.ID
		}
	}

	if clientChanged {
		err = s.q(ctx).UpdateProjectWithClient(ctx, projectdb.UpdateProjectWithClientParams{
			ID:            id,
			Name:          name,
			Description:   descPtr,
			RepositoryUrl: repoPtr,
			Settings:      settingsJSON,
			ClientID:      clientID,
		})
	} else {
		err = s.q(ctx).UpdateProject(ctx, projectdb.UpdateProjectParams{
			ID:            id,
			Name:          name,
			Description:   descPtr,
			RepositoryUrl: repoPtr,
			Settings:      settingsJSON,
		})
	}
	if err != nil {
		return nil, err
	}

	p, err := s.GetByID(ctx, id)
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
	if err := s.q(ctx).SoftDeleteProject(ctx, id); err != nil {
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

// rowQuerier abstrae pool y tx para queries de solo lectura (pgx.Tx y
// pgxpool.Pool lo satisfacen). HasData usa el tx del context si hay (respeta RLS).
type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (s *Service) querier(ctx context.Context) rowQuerier {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return tx
	}
	return s.Pool
}

// HasData reporta si el proyecto tiene contenido real (observations, tickets,
// knowledge, skills, policies, prompts, workflows). Guard de
// domain_project_delete (DOMAINSERV-93): un proyecto con datos no se borra sin
// force. Excluye logs/índices/sesiones (derivados/efímeros).
func (s *Service) HasData(ctx context.Context, id uuid.UUID) (bool, error) {
	const q = `SELECT
		EXISTS(SELECT 1 FROM knowledge_observations WHERE project_id = $1)
		OR EXISTS(SELECT 1 FROM project_tickets WHERE project_id = $1)
		OR EXISTS(SELECT 1 FROM knowledge_docs WHERE project_id = $1)
		OR EXISTS(SELECT 1 FROM project_skills WHERE project_id = $1)
		OR EXISTS(SELECT 1 FROM project_policies WHERE project_id = $1)
		OR EXISTS(SELECT 1 FROM prompts WHERE project_id = $1)
		OR EXISTS(SELECT 1 FROM workflows WHERE project_id = $1)`
	var has bool
	if err := s.querier(ctx).QueryRow(ctx, q, id).Scan(&has); err != nil {
		return false, err
	}
	return has, nil
}

// rowToProject convierte los campos de una fila sqlc al tipo de dominio Project.
func rowToProject(
	id uuid.UUID, name, slug, description, repositoryURL string,
	templateID *uuid.UUID,
	settings []byte,
	clientID *uuid.UUID,
	clientSlug, clientName string,
	createdAt, updatedAt time.Time,
	deletedAt pgtype.Timestamptz,
) *Project {
	var settingsMap map[string]any
	if len(settings) > 0 {
		_ = json.Unmarshal(settings, &settingsMap)
	}
	if settingsMap == nil {
		settingsMap = map[string]any{}
	}
	p := &Project{
		ID:            id,
		Name:          name,
		Slug:          slug,
		Description:   description,
		RepositoryURL: repositoryURL,
		TemplateID:    templateID,
		Settings:      settingsMap,
		ClientID:      clientID,
		ClientSlug:    clientSlug,
		ClientName:    clientName,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}
	if deletedAt.Valid {
		t := deletedAt.Time
		p.DeletedAt = &t
	}
	return p
}

func mapNotFound(err error) error {
	if err == nil {
		return nil
	}
	// pgx retorna pgx.ErrNoRows
	if strings.Contains(err.Error(), "no rows") {
		return ErrNotFound
	}
	return err
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
