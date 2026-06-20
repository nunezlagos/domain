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
	List(ctx context.Context, orgID uuid.UUID, f ListFilter) ([]Project, error)
	Update(ctx context.Context, id uuid.UUID, in UpdateParams) (*Project, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// ListFilter (REQ-28.2): filtros opcionales para List.
type ListFilter struct {
	// ClientID — si non-nil, filtra por projects.client_id = ClientID.
	ClientID *uuid.UUID
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
	// ClientID (REQ-28.2): puede ser nil (proyecto interno). El trigger
	// 000100 valida que pertenezca a la misma organization.
	ClientID *uuid.UUID
}

// UpdateParams: name + opcionales para los demás campos.
type UpdateParams struct {
	Name          string
	Description   string
	RepositoryURL string
	SettingsJSON  []byte
	// ClientID + ClientChanged (REQ-28.2): si ClientChanged == false, el
	// UPDATE no toca client_id (mantiene valor previo). Si true, asigna
	// ClientID (que puede ser nil para unset).
	ClientID      *uuid.UUID
	ClientChanged bool
}
