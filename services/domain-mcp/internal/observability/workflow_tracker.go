// Package observability: este archivo cubre el WorkflowTracker, que
// persiste el lifecycle de cada workflow (start, touch, close, idle).
//
// issue-53.8 workflow-correlation.
package observability

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WorkflowStatus enum CHEKado en BD.
type WorkflowStatus string

const (
	WorkflowRunning   WorkflowStatus = "running"
	WorkflowCompleted WorkflowStatus = "completed"
	WorkflowFailed    WorkflowStatus = "failed"
	WorkflowAbandoned WorkflowStatus = "abandoned"
)

// WorkflowRow es el row completo de workflows.
type WorkflowRow struct {
	ID              uuid.UUID
	Name            string
	Status          WorkflowStatus
	StartedAt       time.Time
	EndedAt         *time.Time
	TotalToolCalls  int
	TotalErrors     int
	TotalDurationMS int64
	ActorID         uuid.UUID
	APIKeyID        uuid.UUID
	ProjectID       uuid.UUID
	LastActivityAt  time.Time
}

// WorkflowStore abstrae la persistencia del lifecycle de workflows.
type WorkflowStore interface {
	UpsertWorkflow(ctx context.Context, w WorkflowRow) error
	MarkWorkflowIdle(ctx context.Context, olderThan time.Duration) (int, error)
	GetWorkflow(ctx context.Context, id uuid.UUID) (WorkflowRow, error)
}

// PGWorkflowStore implementa WorkflowStore contra postgres workflows table.
type PGWorkflowStore struct {
	Pool *pgxpool.Pool
}

// SetPool atacha el pool (post db.Open*).
func (s *PGWorkflowStore) SetPool(p *pgxpool.Pool) { s.Pool = p }

// UpsertWorkflow hace INSERT ON CONFLICT UPDATE.
func (s *PGWorkflowStore) UpsertWorkflow(ctx context.Context, w WorkflowRow) error {
	if s.Pool == nil {
		return ErrStoreNotReady
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO workflows (
			id, name, status, started_at, ended_at,
			total_tool_calls, total_errors, total_duration_ms,
			actor_id, api_key_id, project_id, last_activity_at, metadata
		) VALUES ($1, NULLIF($2,''), $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, '{}'::jsonb)
		ON CONFLICT (id) DO UPDATE SET
			last_activity_at = EXCLUDED.last_activity_at,
			total_tool_calls = workflows.total_tool_calls + EXCLUDED.total_tool_calls,
			total_errors = workflows.total_errors + EXCLUDED.total_errors,
			total_duration_ms = workflows.total_duration_ms + EXCLUDED.total_duration_ms
	`,
		w.ID, w.Name, string(w.Status), w.StartedAt, w.EndedAt,
		w.TotalToolCalls, w.TotalErrors, w.TotalDurationMS,
		nullableUUID(w.ActorID), nullableUUID(w.APIKeyID), nullableUUID(w.ProjectID),
		w.LastActivityAt,
	)
	return err
}

// MarkWorkflowIdle marca workflows running con last_activity_at < threshold como abandoned.
// Devuelve el numero de rows afectados.
func (s *PGWorkflowStore) MarkWorkflowIdle(ctx context.Context, olderThan time.Duration) (int, error) {
	if s.Pool == nil {
		return 0, ErrStoreNotReady
	}
	threshold := time.Now().Add(-olderThan)
	tag, err := s.Pool.Exec(ctx, `
		UPDATE workflows
		SET status = 'abandoned', ended_at = last_activity_at
		WHERE status = 'running' AND last_activity_at < $1
	`, threshold)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// GetWorkflow devuelve el row actual de workflows.
func (s *PGWorkflowStore) GetWorkflow(ctx context.Context, id uuid.UUID) (WorkflowRow, error) {
	if s.Pool == nil {
		return WorkflowRow{}, ErrStoreNotReady
	}
	var (
		w       WorkflowRow
		status  string
		name    *string
		actor   *uuid.UUID
		apiKey  *uuid.UUID
		project *uuid.UUID
	)
	err := s.Pool.QueryRow(ctx, `
		SELECT id, name, status, started_at, ended_at,
			total_tool_calls, total_errors, total_duration_ms,
			actor_id, api_key_id, project_id, last_activity_at
		FROM workflows WHERE id = $1
	`, id).Scan(&w.ID, &name, &status, &w.StartedAt, &w.EndedAt,
		&w.TotalToolCalls, &w.TotalErrors, &w.TotalDurationMS,
		&actor, &apiKey, &project, &w.LastActivityAt)
	if err != nil {
		return WorkflowRow{}, err
	}
	w.Status = WorkflowStatus(status)
	if name != nil {
		w.Name = *name
	}
	if actor != nil {
		w.ActorID = *actor
	}
	if apiKey != nil {
		w.APIKeyID = *apiKey
	}
	if project != nil {
		w.ProjectID = *project
	}
	return w, nil
}

// Tracker gestiona el lifecycle de workflows corriendo en el proceso.
// Touch() actualiza last_activity_at y counters en cada tool invocation.
// Heartbeat cada idleCheckInterval cierra workflows abandoned.
type Tracker struct {
	store             WorkflowStore
	logger            *slog.Logger
	idleTimeout       time.Duration
	idleCheckInterval time.Duration
	heartbeatCtx      context.Context
	heartbeatCancel   context.CancelFunc
	heartbeatDone     chan struct{}
	once              sync.Once
}

// TrackerIdleDefault es el default para marcar workflow como abandoned.
const TrackerIdleDefault = 5 * time.Minute

// TrackerIntervalDefault es el intervalo del heartbeat.
const TrackerIntervalDefault = 1 * time.Minute

// NewTracker construye un tracker. Llamar Start() para activar el heartbeat.
func NewTracker(store WorkflowStore, logger *slog.Logger, idle time.Duration, interval time.Duration) *Tracker {
	if idle <= 0 {
		idle = TrackerIdleDefault
	}
	if interval <= 0 {
		interval = TrackerIntervalDefault
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Tracker{
		store:             store,
		logger:            logger,
		idleTimeout:       idle,
		idleCheckInterval: interval,
	}
}

// Start arranca el heartbeat goroutine.
func (t *Tracker) Start(parent context.Context) {
	t.once.Do(func() {
		t.heartbeatCtx, t.heartbeatCancel = context.WithCancel(parent)
		t.heartbeatDone = make(chan struct{})
		go t.heartbeatLoop()
	})
}

// Stop cancela el heartbeat y espera. Idempotente.
func (t *Tracker) Stop() {
	if t.heartbeatCancel != nil {
		t.heartbeatCancel()
	}
	if t.heartbeatDone != nil {
		<-t.heartbeatDone
	}
}

func (t *Tracker) heartbeatLoop() {
	defer close(t.heartbeatDone)
	tk := time.NewTicker(t.idleCheckInterval)
	defer tk.Stop()
	for {
		select {
		case <-t.heartbeatCtx.Done():
			return
		case <-tk.C:
			t.tickIdle()
		}
	}
}

func (t *Tracker) tickIdle() {
	ctx, cancel := context.WithTimeout(t.heartbeatCtx, defaultTimeout)
	defer cancel()
	n, err := t.store.MarkWorkflowIdle(ctx, t.idleTimeout)
	if err != nil {
		t.logger.Warn("workflow idle mark failed", slog.String("error", err.Error()))
		return
	}
	if n > 0 {
		t.logger.Info("workflows marked abandoned",
			slog.Int("count", n),
			slog.String("idle_minutes", t.idleTimeout.String()))
	}
}

// Touch actualiza last_activity_at y counters del workflow en BD.
// Llamar al final de cada tool invocation o HTTP request.
func (t *Tracker) Touch(ctx context.Context, w WorkflowRow) {
	if w.ID == uuid.Nil {
		return
	}
	w.LastActivityAt = time.Now()
	bgCtx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	if err := t.store.UpsertWorkflow(bgCtx, w); err != nil {
		t.logger.Warn("workflow touch failed",
			slog.String("workflow_id", w.ID.String()),
			slog.String("error", err.Error()))
	}
}
