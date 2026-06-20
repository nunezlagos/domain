// Package flow — HU-28.1 Repository interface.
//
// Cubre las operaciones de Service.CRUD + flow_runs (list/pause/resume/cancel/get).
// Otros stores del package (SignalStore, VersioningStore, SagaStore,
// HeartbeatStore) NO se migran en esta HU; siguen usando *pgxpool.Pool directo.
package flow

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository abstrae acceso a tablas flows + flow_runs + flow_run_steps.
type Repository interface {
	InsertFlow(ctx context.Context, in InsertFlowParams) (*Flow, error)
	UpdateFlow(ctx context.Context, in UpdateFlowParams) (*Flow, error)
	GetFlowByID(ctx context.Context, id uuid.UUID) (*Flow, error)
	GetFlowBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Flow, error)
	ListFlows(ctx context.Context, orgID uuid.UUID, limit int) ([]Flow, error)
	ListParents(ctx context.Context, orgID uuid.UUID, slug string) ([]Flow, error)
	SoftDeleteFlow(ctx context.Context, id uuid.UUID) error

	GetRun(ctx context.Context, id uuid.UUID) (*RunRow, error)
	GetRunSteps(ctx context.Context, runID uuid.UUID) ([]StepRow, error)
	ListRuns(ctx context.Context, f RunFilter) ([]RunRow, int, error)
	PauseRun(ctx context.Context, id uuid.UUID, fromStatus string) error
	ResumeRun(ctx context.Context, id uuid.UUID) error
	CancelRun(ctx context.Context, id uuid.UUID, finishedAt time.Time) error
}

// InsertFlowParams agrupa params del INSERT (spec ya viene serializado a JSON).
type InsertFlowParams struct {
	OrganizationID      uuid.UUID
	Slug                string
	Name                string
	Description         string
	SpecJSON            []byte
	DeterministicReplay bool
}

// UpdateFlowParams agrupa params del UPDATE. Optimistic locking via
// ExpectedUpdatedAt (si != nil, agrega AND updated_at = $7).
type UpdateFlowParams struct {
	ID                uuid.UUID
	Name              string
	Description       string
	SpecJSON          []byte
	IsActive          bool
	IsUserModified    bool
	ExpectedUpdatedAt *time.Time // nil = sin optimistic lock
}
