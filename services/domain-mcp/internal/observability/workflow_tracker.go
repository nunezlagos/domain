// Package observability: este archivo cubre el WorkflowTracker, que
// persiste el lifecycle de cada workflow (start, touch, close, idle).
//
// issue-53.8 workflow-correlation.
package observability

import (
	"context"
	"fmt"
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
	WorkflowCancelled WorkflowStatus = "cancelled"
)

// TerminalWorkflowStatus traduce un status de flow_runs al de workflows y dice si
// es terminal.
//
// Desde la migración 000283 el mapeo es IDENTIDAD para los tres estados
// terminales, así que no hay traducción en la que equivocarse: la función existe
// para RECHAZAR lo que no es terminal, no para convertir. Devolver ok=false ante
// un status desconocido es deliberado — antes que inventar un cierre, no cerrar.
func TerminalWorkflowStatus(flowStatus string) (WorkflowStatus, bool) {
	switch flowStatus {
	case string(WorkflowCompleted):
		return WorkflowCompleted, true
	case string(WorkflowFailed):
		return WorkflowFailed, true
	case string(WorkflowCancelled):
		return WorkflowCancelled, true
	default:
		return "", false
	}
}

// WorkflowRow es el row completo de workflows.
//
// ActorID, APIKeyID y ProjectID son punteros porque las tres columnas son
// NULLables y "no hay actor" no es lo mismo que el uuid en ceros: con un
// no-puntero, un NULL de la BD queda indistinguible de un centinela real al
// serializar (DOMAINSERV-229). Mismo criterio que audit.AuditEntry.
type WorkflowRow struct {
	ID              uuid.UUID
	Name            string
	Status          WorkflowStatus
	StartedAt       time.Time
	EndedAt         *time.Time
	TotalToolCalls  int
	TotalErrors     int
	TotalDurationMS int64
	ActorID         *uuid.UUID
	APIKeyID        *uuid.UUID
	ProjectID       *uuid.UUID
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
	// started_at: si el caller no seteó StartedAt (zero-value de Go =
	// 0001-01-01), pasamos NULL para que actúe el DEFAULT now() de la
	// columna. Pasar el zero-value directo lo persistía como timestamp
	// basura (era un valor NOT NULL válido, así que el DEFAULT nunca
	// disparaba). El ON CONFLICT además repara filas que ya quedaron en
	// zero por el bug previo: COALESCE toma el primer started_at real.
	// project_id: el caller no lo tiene. Touch arma la fila desde el ctx del hook de
	// metricas, y las tools que declaran flow_run_id no traen el proyecto en sus args.
	// Se deriva de flow_runs, que es legitimo porque el workflow_id ES el flow_run_id
	// (DOMAINSERV-212). Un workflow que no viene de una corrida no matchea y queda NULL:
	// inventarlo seria peor que no tenerlo. El COALESCE del ON CONFLICT ademas repara las
	// filas que ya nacieron sin el.
	startedAt := startedAtOrActivity(w.StartedAt, w.LastActivityAt)
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO workflows (
			id, name, status, started_at, ended_at,
			total_tool_calls, total_errors, total_duration_ms,
			actor_id, api_key_id, project_id, last_activity_at, metadata
		) VALUES ($1, NULLIF($2,''), $3, COALESCE($4, now()), $5, $6, $7, $8, $9, $10,
			COALESCE($11, (SELECT project_id FROM flow_runs WHERE id = $1)), $12, '{}'::jsonb)
		ON CONFLICT (id) DO UPDATE SET
			project_id = COALESCE(workflows.project_id, EXCLUDED.project_id),
			started_at = CASE
				WHEN workflows.started_at = 'epoch'::timestamptz OR workflows.started_at < '1970-01-01'::timestamptz
					THEN COALESCE(EXCLUDED.started_at, now())
				ELSE workflows.started_at
			END,
			-- El DO UPDATE no tocaba status ni ended_at, así que el cierre terminal
			-- se perdía en silencio y una fila que el reaper dio por 'abandoned' no
			-- podía volver a 'running' por más Touch que recibiera: quedaba muerta
			-- con su ended_at congelado mientras seguía consumiendo tool calls
			-- (DOMAINSERV-230, 8 de 20 filas en prod con ended_at < last_activity_at).
			status = CASE
				-- un cierre terminal es la única señal autoritativa del final y manda
				-- siempre; en particular puede llegar ANTES de que exista la fila, y
				-- entonces un Touch posterior no debe reabrirla
				WHEN EXCLUDED.status <> 'running' THEN EXCLUDED.status
				-- una fila dada por muerta que vuelve a recibir actividad revive,
				-- PERO solo si su corrida sigue viva: sin el NOT EXISTS, una tool
				-- call rezagada resucita como 'running' el workflow de un flow_run
				-- que ya cerró, y eso es peor que el 'abandoned' que venía a arreglar
				WHEN workflows.status = 'abandoned' AND NOT EXISTS (
					SELECT 1 FROM flow_runs f
					WHERE f.id = workflows.id
					  AND f.status IN ('completed', 'failed', 'cancelled')
				) THEN EXCLUDED.status
				ELSE workflows.status
			END,
			ended_at = CASE
				WHEN EXCLUDED.status <> 'running' THEN COALESCE(EXCLUDED.ended_at, EXCLUDED.last_activity_at)
				WHEN workflows.status = 'abandoned' AND NOT EXISTS (
					SELECT 1 FROM flow_runs f
					WHERE f.id = workflows.id
					  AND f.status IN ('completed', 'failed', 'cancelled')
				) THEN NULL
				ELSE workflows.ended_at
			END,
			last_activity_at = EXCLUDED.last_activity_at,
			total_tool_calls = workflows.total_tool_calls + EXCLUDED.total_tool_calls,
			total_errors = workflows.total_errors + EXCLUDED.total_errors,
			total_duration_ms = workflows.total_duration_ms + EXCLUDED.total_duration_ms
	`,
		w.ID, w.Name, string(w.Status), startedAt, w.EndedAt,
		w.TotalToolCalls, w.TotalErrors, w.TotalDurationMS,
		nullableUUIDPtr(w.ActorID), nullableUUIDPtr(w.APIKeyID), nullableUUIDPtr(w.ProjectID),
		w.LastActivityAt,
	)
	return err
}

// nullableStartedAt convierte un StartedAt en el bind param correcto para
// el INSERT: nil (→ SQL NULL, que dispara el DEFAULT now()) si el valor
// es el zero-value de Go, o el timestamp real en caso contrario. Extraído
// para poder testear la decisión sin una BD real.
func nullableStartedAt(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// startedAtOrActivity elige el started_at del INSERT usando UN SOLO reloj.
//
// Antes, un workflow de un solo Touch caía en el COALESCE($4, now()) y su
// started_at lo ponía Postgres al ejecutar el statement, mientras que
// last_activity_at venía del time.Now() de Go calculado ANTES de despachar la
// query — y MarkWorkflowIdle hace ended_at = last_activity_at. Resultado:
// started_at SIEMPRE posterior a ended_at, y siempre con el mismo signo (−25 ms,
// −3 ms…), o sea que no era skew de relojes sino orden de ejecución
// (DOMAINSERV-230). Duraciones negativas por construcción.
//
// El now() de la columna queda como último recurso, para el caller que no trae
// ninguno de los dos.
func startedAtOrActivity(startedAt, lastActivity time.Time) *time.Time {
	if !startedAt.IsZero() {
		return &startedAt
	}
	if !lastActivity.IsZero() {
		return &lastActivity
	}
	return nil
}

// CloseWorkflow lleva el workflow de una corrida a su estado terminal.
//
// Reusa UpsertWorkflow en vez de un UPDATE porque el cierre puede llegar antes de
// que exista la fila (DOMAINSERV-230): un UPDATE no-operaría en silencio y el
// workflow quedaría sin cerrar para siempre. Los contadores van en 0 porque el
// Upsert los SUMA — un cierre no aporta tool calls, solo el final.
//
// last_activity_at se setea al mismo endedAt para no dejar una fila cuyo último
// latido sea posterior a su propio cierre, que es la inconsistencia que este
// ticket midió en 8 de 20 filas de producción.
func (s *PGWorkflowStore) CloseWorkflow(ctx context.Context, flowRunID uuid.UUID, status string, endedAt time.Time) error {
	st, ok := TerminalWorkflowStatus(status)
	if !ok {
		return fmt.Errorf("observability: %q no es un estado terminal de workflow", status)
	}
	return s.UpsertWorkflow(ctx, WorkflowRow{
		ID:             flowRunID,
		Status:         st,
		EndedAt:        &endedAt,
		LastActivityAt: endedAt,
	})
}

// MarkWorkflowIdle marca workflows running con last_activity_at < threshold como abandoned.
// Devuelve el numero de rows afectados.
func (s *PGWorkflowStore) MarkWorkflowIdle(ctx context.Context, olderThan time.Duration) (int, error) {
	if s.Pool == nil {
		return 0, ErrStoreNotReady
	}
	threshold := time.Now().Add(-olderThan)
	// GREATEST y no last_activity_at pelado: una fila cuyo started_at quedó
	// adelantado por el bug de los dos relojes (DOMAINSERV-230) dejaría un
	// ended_at anterior a su propio inicio, o sea una duración negativa. El
	// GREATEST es defensa en profundidad para las filas que ya nacieron torcidas:
	// el fix de startedAtOrActivity evita las nuevas, no repara las viejas.
	tag, err := s.Pool.Exec(ctx, `
		UPDATE workflows
		SET status = 'abandoned', ended_at = GREATEST(last_activity_at, started_at)
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
		w      WorkflowRow
		status string
		name   *string
	)
	err := s.Pool.QueryRow(ctx, `
		SELECT id, name, status, started_at, ended_at,
			total_tool_calls, total_errors, total_duration_ms,
			actor_id, api_key_id, project_id, last_activity_at
		FROM workflows WHERE id = $1
	`, id).Scan(&w.ID, &name, &status, &w.StartedAt, &w.EndedAt,
		&w.TotalToolCalls, &w.TotalErrors, &w.TotalDurationMS,
		&w.ActorID, &w.APIKeyID, &w.ProjectID, &w.LastActivityAt)
	if err != nil {
		return WorkflowRow{}, err
	}
	w.Status = WorkflowStatus(status)
	if name != nil {
		w.Name = *name
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
