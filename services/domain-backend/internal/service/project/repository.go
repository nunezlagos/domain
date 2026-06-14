// Package project — HU-28.1 Repository interface.
package project

import (
	"context"

	"github.com/google/uuid"
)

// Repository abstrae acceso a la tabla projects.
type Repository interface {
	Insert(ctx context.Context, in InsertParams) (*Project, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Project, error)
	GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Project, error)
	List(ctx context.Context, orgID uuid.UUID) ([]Project, error)
	Update(ctx context.Context, id uuid.UUID, in UpdateParams) (*Project, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// InsertParams agrupa campos del INSERT (settings ya viene como JSON serializado).
type InsertParams struct {
	OrganizationID uuid.UUID
	Name           string
	Slug           string
	Description    string
	RepositoryURL  string
	TemplateID     *uuid.UUID
	SettingsJSON   []byte
}

// UpdateParams: name + opcionales para los demás campos.
type UpdateParams struct {
	Name          string
	Description   string
	RepositoryURL string
	SettingsJSON  []byte
}
