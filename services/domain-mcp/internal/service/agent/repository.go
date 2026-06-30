// Package agent — HU-28.1 Repository interface.
package agent

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository abstrae acceso a las tablas agents + agent_versions + tablas
// referenciadas para validación (skills).
type Repository interface {
	Insert(ctx context.Context, in InsertParams) (*Agent, error)
	Update(ctx context.Context, id uuid.UUID, in UpdateParams) (*Agent, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Agent, error)
	GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Agent, error)
	List(ctx context.Context, orgID uuid.UUID, limit int) ([]Agent, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error



	CountValidSkills(ctx context.Context, orgID uuid.UUID, slugs []string) (int, error)


	SlugTaken(ctx context.Context, orgID uuid.UUID, slug string) (bool, error)


	ArchiveVersion(ctx context.Context, in ArchiveVersionParams) error


	ListVersions(ctx context.Context, agentID uuid.UUID, limit int) ([]AgentVersion, error)
}

// InsertParams agrupa params del INSERT en agents.
type InsertParams struct {
	OrganizationID uuid.UUID
	Slug           string
	Name           string
	Description    string
	Provider       string
	Model          string
	SystemPrompt   string
	SkillsSlugs    []string
	MaxIterations  int
	TokenBudget    *int64
	Temperature    *float64
}

// UpdateParams es el conjunto de campos que llegan al UPDATE.
type UpdateParams struct {
	Name           string
	Description    string
	Provider       string
	Model          string
	SystemPrompt   string
	SkillsSlugs    []string
	MaxIterations  int
	TokenBudget    *int64
	Temperature    *float64
	IsUserModified bool
}

// ArchiveVersionParams: snapshot serializado del agent previo + actor.
type ArchiveVersionParams struct {
	AgentID         uuid.UUID
	Snapshot        map[string]any
	ChangedBy       *uuid.UUID
	MaxVersionsKept int
}

var _ = time.Time{}
