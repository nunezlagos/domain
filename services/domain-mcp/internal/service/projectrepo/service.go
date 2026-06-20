package projectrepo

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrNameRequired = errors.New("project_repo: name requerido")
	ErrURLRequired  = errors.New("project_repo: url requerida")
)

var validWorkflows = map[string]struct{}{
	"":             {},
	"merge":        {},
	"pr":           {},
	"mr":           {},
	"trunk_based":  {},
	"rebase":       {},
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

type AddInput struct {
	ProjectID      uuid.UUID
	Name           string
	URL            string
	BranchDefault  string
	Kind           string
	IsDefault      bool
	Workflow       string
	Notes          string
}

func (s *Service) Add(ctx context.Context, in AddInput) (*Repo, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.URL = strings.TrimSpace(in.URL)
	if in.Name == "" {
		return nil, ErrNameRequired
	}
	if in.URL == "" {
		return nil, ErrURLRequired
	}
	if _, ok := validWorkflows[strings.ToLower(strings.TrimSpace(in.Workflow))]; !ok {
		return nil, ErrInvalidWorkflow
	}
	// Si es el primer repo del proyecto, lo marcamos default por defecto.
	existing, _ := s.repo.List(ctx, uuid.Nil, in.ProjectID)
	if len(existing) == 0 {
		in.IsDefault = true
	}
	created, err := s.repo.Insert(ctx, InsertParams{
		ProjectID:      in.ProjectID,
		Name:           in.Name,
		URL:            in.URL,
		BranchDefault:  strings.TrimSpace(in.BranchDefault),
		Kind:           strings.ToLower(strings.TrimSpace(in.Kind)),
		IsDefault:      in.IsDefault,
		Workflow:       strings.ToLower(strings.TrimSpace(in.Workflow)),
		Notes:          strings.TrimSpace(in.Notes),
	})
	if err != nil {
		return nil, err
	}
	// Si IsDefault=true y había otros, hay que limpiar el previo via SetDefault.
	if created.IsDefault && len(existing) > 0 {
		updated, derr := s.repo.SetDefault(ctx, uuid.Nil, created.ID)
		if derr == nil {
			return updated, nil
		}
	}
	return created, nil
}

func (s *Service) List(ctx context.Context, orgID, projectID uuid.UUID) ([]*Repo, error) {
	return s.repo.List(ctx, orgID, projectID)
}

func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (*Repo, error) {
	return s.repo.Get(ctx, orgID, id)
}

func (s *Service) GetByName(ctx context.Context, orgID, projectID uuid.UUID, name string) (*Repo, error) {
	return s.repo.GetByName(ctx, orgID, projectID, strings.TrimSpace(name))
}

func (s *Service) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateParams) (*Repo, error) {
	if in.Workflow != nil {
		w := strings.ToLower(strings.TrimSpace(*in.Workflow))
		if _, ok := validWorkflows[w]; !ok {
			return nil, ErrInvalidWorkflow
		}
		in.Workflow = &w
	}
	return s.repo.Update(ctx, orgID, id, in)
}

func (s *Service) SetDefault(ctx context.Context, orgID, id uuid.UUID) (*Repo, error) {
	return s.repo.SetDefault(ctx, orgID, id)
}

func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return s.repo.SoftDelete(ctx, orgID, id)
}
