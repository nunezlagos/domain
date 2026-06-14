package projectpolicy

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

var validKinds = map[string]struct{}{
	"convention":     {}, "security_rule": {}, "architecture": {},
	"sdd_workflow":   {}, "observability": {}, "migration_rule": {},
	"linter_config":  {}, "agent_protocol": {}, "git_workflow": {},
	"tech_stack":     {}, "test_strategy": {},
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Policy, error) {
	in.Slug = strings.TrimSpace(in.Slug)
	in.Name = strings.TrimSpace(in.Name)
	in.Kind = strings.TrimSpace(in.Kind)
	if in.Slug == "" {
		return nil, ErrSlugRequired
	}
	if _, ok := validKinds[in.Kind]; !ok {
		return nil, ErrInvalidKind
	}
	return s.repo.Insert(ctx, in)
}

func (s *Service) List(ctx context.Context, orgID, projectID uuid.UUID, kind string) ([]*Policy, error) {
	return s.repo.List(ctx, orgID, projectID, strings.TrimSpace(kind))
}

func (s *Service) GetBySlug(ctx context.Context, orgID, projectID uuid.UUID, slug string) (*Policy, error) {
	return s.repo.GetBySlug(ctx, orgID, projectID, strings.TrimSpace(slug))
}

func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (*Policy, error) {
	return s.repo.Get(ctx, orgID, id)
}

func (s *Service) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, changedBy *uuid.UUID) (*Policy, error) {
	if in.Kind != nil {
		k := strings.TrimSpace(*in.Kind)
		if _, ok := validKinds[k]; !ok {
			return nil, ErrInvalidKind
		}
		in.Kind = &k
	}
	return s.repo.Update(ctx, orgID, id, in, changedBy)
}

func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return s.repo.SoftDelete(ctx, orgID, id)
}
