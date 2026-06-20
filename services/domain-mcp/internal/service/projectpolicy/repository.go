// Package projectpolicy — REQ-43 policies scoped a (org, project).
// Resolver jerárquico: project_policies → fallback platform_policies.
//
// Cuando un proyecto tiene una policy con mismo slug que la platform:
//   override_platform=true  → la project_policy reemplaza
//   override_platform=false → el LLM ve ambas (project + platform)
//                              concatenadas. Default = false (amplía).
package projectpolicy

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound     = errors.New("project_policy not found")
	ErrInvalidKind  = errors.New("project_policy: kind inválido")
	ErrSlugRequired = errors.New("project_policy: slug requerido")
)

type Policy struct {
	ID               uuid.UUID  `json:"id"`
	OrganizationID   uuid.UUID  `json:"organization_id"`
	ProjectID        uuid.UUID  `json:"project_id"`
	Slug             string     `json:"slug"`
	Name             string     `json:"name"`
	Kind             string     `json:"kind"`
	BodyMD           string     `json:"body_md"`
	BodyStructured   any        `json:"body_structured,omitempty"`
	Version          int        `json:"version"`
	IsActive         bool       `json:"is_active"`
	OverridePlatform bool       `json:"override_platform"`
	Source           string     `json:"source"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

type CreateInput struct {
	OrganizationID   uuid.UUID
	ProjectID        uuid.UUID
	Slug             string
	Name             string
	Kind             string
	BodyMD           string
	BodyStructured   any
	OverridePlatform bool
	Source           string // manual | llm_generated | seed_imported | dashboard
}

type UpdateInput struct {
	Name             *string
	Kind             *string
	BodyMD           *string
	BodyStructured   any
	OverridePlatform *bool
}

type Repository interface {
	Insert(ctx context.Context, in CreateInput) (*Policy, error)
	List(ctx context.Context, orgID, projectID uuid.UUID, kind string) ([]*Policy, error)
	GetBySlug(ctx context.Context, orgID, projectID uuid.UUID, slug string) (*Policy, error)
	Get(ctx context.Context, orgID, id uuid.UUID) (*Policy, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput, changedBy *uuid.UUID) (*Policy, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID) error
}
