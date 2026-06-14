// Package projectrepo — REQ-42 multi-remotos por proyecto. CRUD + helpers
// para que el LLM (o el usuario vía dashboard) pueda registrar varios
// remotos por proyecto, marcar uno como default, y resolver "qué remoto
// + rama usar" sin dejar decisiones implícitas.
package projectrepo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound        = errors.New("project_repo not found")
	ErrDuplicateName   = errors.New("project_repo name already exists for this project")
	ErrInvalidWorkflow = errors.New("project_repo workflow inválido (merge|pr|mr|trunk_based)")
)

type Repo struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	ProjectID      uuid.UUID  `json:"project_id"`
	Name           string     `json:"name"`
	URL            string     `json:"url"`
	BranchDefault  string     `json:"branch_default,omitempty"`
	Kind           string     `json:"kind,omitempty"`
	IsDefault      bool       `json:"is_default"`
	Workflow       string     `json:"workflow,omitempty"`
	Notes          string     `json:"notes,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
}

type InsertParams struct {
	OrganizationID uuid.UUID
	ProjectID      uuid.UUID
	Name           string
	URL            string
	BranchDefault  string
	Kind           string
	IsDefault      bool
	Workflow       string
	Notes          string
}

type UpdateParams struct {
	URL           *string
	BranchDefault *string
	Kind          *string
	Workflow      *string
	Notes         *string
}

type Repository interface {
	Insert(ctx context.Context, in InsertParams) (*Repo, error)
	List(ctx context.Context, orgID, projectID uuid.UUID) ([]*Repo, error)
	Get(ctx context.Context, orgID, id uuid.UUID) (*Repo, error)
	GetByName(ctx context.Context, orgID, projectID uuid.UUID, name string) (*Repo, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateParams) (*Repo, error)
	SetDefault(ctx context.Context, orgID, id uuid.UUID) (*Repo, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID) error
}
