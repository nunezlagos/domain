// Package client — REQ-28.1 service + repository de clients.
//
// Clients representa cuentas/empresas externas que la organización gestiona
// como contraparte (clientes finales, partners, contratantes). Aislado por
// organization_id (multi-tenant, RLS en DB). Slug único per-org.
//
// repository.go define el contrato Repository (interface), el modelo Client,
// los params de Insert/Update y los errores de dominio. Service depende de
// esta interfaz (no de *pgxpool.Pool directo) → la lógica de negocio
// (validaciones, audit) es unit-testeable con mocks.
package client

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Errores de dominio. Se exportan para que el handler HTTP pueda mapearlos
// a códigos de status (404 / 409 / 422 / etc).
var (


	ErrClientNotFound = errors.New("client not found")


	ErrClientSlugExists = errors.New("client slug already exists in this organization")

	ErrInvalidStatus = errors.New("invalid client status (must be active|inactive|archived)")

	ErrInvalidSlug = errors.New("invalid slug (must be lowercase kebab-case)")

	ErrInvalidName = errors.New("name must have at least 2 chars")

	ErrInvalidEmail = errors.New("invalid contact email format")

	ErrInvalidTaxID = errors.New("invalid tax_id format")
)

// Status enum (mirror del CHECK constraint en DB).
const (
	StatusActive   = "active"
	StatusInactive = "inactive"
	StatusArchived = "archived"
)

// Client es la representación in-memory de la row de clients.
type Client struct {
	ID             uuid.UUID
	Name           string
	Slug           string
	TaxID          string // "" si NULL
	ContactEmail   string
	ContactPhone   string
	Address        string
	Metadata       map[string]any
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

// ClientUpdate agrupa los campos parchables. Punteros distinguen "no tocar"
// (nil) de "set explicit value" (no-nil, incluso si es "").
type ClientUpdate struct {
	Name         *string
	TaxID        *string
	ContactEmail *string
	ContactPhone *string
	Address      *string
	Metadata     map[string]any // nil = no tocar; non-nil = reemplazar completo
	Status       *string
}

// InsertParams agrupa los campos requeridos por el INSERT, ya validados
// por el Service.
type InsertParams struct {
	Name           string
	Slug           string
	TaxID          string
	ContactEmail   string
	ContactPhone   string
	Address        string
	MetadataJSON   []byte
	Status         string
}

// UpdateParams agrupa el conjunto resuelto de campos a actualizar (Service
// ya hizo merge entre prev + parche).
type UpdateParams struct {
	Name         string
	TaxID        string
	ContactEmail string
	ContactPhone string
	Address      string
	MetadataJSON []byte
	Status       string
}

// ListFilter filtra el List + paginación simple (offset/limit).
type ListFilter struct {
	Status         string // "" = todos
	Search         string // ILIKE sobre name/slug
	IncludeDeleted bool   // si true, incluye soft-deleted
	Limit          int
	Offset         int
}

// Repository abstrae el acceso a la tabla clients.
//
// La implementación concreta (pg_repository.go) honra tx-context: si el ctx
// trae una tx inyectada por el middleware HTTP, las queries corren contra esa
// tx (RLS activa).
type Repository interface {



	Insert(ctx context.Context, in InsertParams) (*Client, error)



	GetByID(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*Client, error)


	GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Client, error)




	List(ctx context.Context, orgID uuid.UUID, f ListFilter) ([]*Client, int64, error)



	Update(ctx context.Context, orgID uuid.UUID, id uuid.UUID, in UpdateParams) (*Client, error)



	SoftDelete(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error



	Restore(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error



	SetStatus(ctx context.Context, orgID uuid.UUID, id uuid.UUID, status string) (*Client, error)
}
