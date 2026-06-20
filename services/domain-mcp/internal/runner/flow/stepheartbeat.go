// issue-09.10 — heartbeats por step con throttle (hb-002) y publicación de
// progreso vía NOTIFY (hb-004). Los steps obtienen el heartbeater desde su
// context con HeartbeaterFrom(ctx) y llaman Beat(progress, message).
package flowrunner

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/flow"
)

// DefaultHeartbeatMinInterval es el throttle entre beats efectivos (hb-002).
const DefaultHeartbeatMinInterval = 5 * time.Second

// StepHeartbeater emite heartbeats de un step en ejecución, con throttle.
// Beats dentro de la ventana MinInterval son no-op (anti-flood).
type StepHeartbeater struct {
	Store       *flow.HeartbeatStore
	RunID       uuid.UUID
	StepRowID   uuid.UUID
	StepKey     string
	MinInterval time.Duration

	mu   sync.Mutex
	last time.Time
}

// shouldBeat aplica el throttle. Separado para testear sin DB.
func (h *StepHeartbeater) shouldBeat(now time.Time) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	min := h.MinInterval
	if min <= 0 {
		min = DefaultHeartbeatMinInterval
	}
	if !h.last.IsZero() && now.Sub(h.last) < min {
		return false
	}
	h.last = now
	return true
}

// Beat actualiza heartbeat + progress del step y publica el evento SSE.
// progress en [0,1]. Throttled: máximo un beat efectivo cada MinInterval.
func (h *StepHeartbeater) Beat(ctx context.Context, progress float64, message string) error {
	if h == nil || h.Store == nil {
		return nil
	}
	if !h.shouldBeat(time.Now()) {
		return nil
	}
	if err := h.Store.BeatWithProgress(ctx, h.StepRowID, progress, message); err != nil {
		return err
	}
	h.Store.NotifyProgress(ctx, h.RunID, h.StepKey, progress, message)
	return nil
}

type heartbeaterCtxKey struct{}

// WithHeartbeater inyecta el heartbeater del step en el context.
func WithHeartbeater(ctx context.Context, h *StepHeartbeater) context.Context {
	return context.WithValue(ctx, heartbeaterCtxKey{}, h)
}

// HeartbeaterFrom recupera el heartbeater del step. Nunca nil: si el context
// no tiene uno, retorna un no-op (Beat retorna nil sin tocar DB).
func HeartbeaterFrom(ctx context.Context) *StepHeartbeater {
	if h, ok := ctx.Value(heartbeaterCtxKey{}).(*StepHeartbeater); ok && h != nil {
		return h
	}
	return &StepHeartbeater{}
}

// beginStepRow crea la fila running del step al iniciar (visible para el
// zombie watchdog y para GET progress). Retorna uuid.Nil si falla (best-effort).
func (r *Runner) beginStepRow(ctx context.Context, runID uuid.UUID, stepKey string) uuid.UUID {
	var id uuid.UUID
	err := r.Pool.QueryRow(ctx, `
		INSERT INTO flow_run_steps
			(flow_run_id, step_key, status, inputs, started_at, last_heartbeat_at)
		VALUES ($1, $2, 'running', '{}', NOW(), NOW())
		RETURNING id`, runID, stepKey,
	).Scan(&id)
	if err != nil {
		return uuid.Nil
	}
	return id
}

// completeStepRow cierra la fila del step con su resultado final.
func (r *Runner) completeStepRow(ctx context.Context, rowID uuid.UUID, output any, stepErr error) error {
	if rowID == uuid.Nil {
		return nil
	}
	status := "completed"
	var errStr *string
	if stepErr != nil {
		status = "failed"
		s := stepErr.Error()
		errStr = &s
	}
	outputsJSON, _ := json.Marshal(output)
	compressed, _, _ := CompressOutput(output)
	_, err := r.Pool.Exec(ctx, `
		UPDATE flow_run_steps
		SET status = $2, outputs = $3, error = $4, output_compressed = $5,
		    completed_at = NOW(), last_heartbeat_at = NOW()
		WHERE id = $1`,
		rowID, status, outputsJSON, errStr, compressed,
	)
	return err
}
