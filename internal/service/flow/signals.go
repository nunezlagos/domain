// issue-09.8 external-signals — inyecta señales externas a flows en ejecución.
//
// Tabla flow_signals (migration 000062) buffer-able. El executor del flow
// polea signals para sus runs activos cada tick; un step puede esperar una
// señal con timeout configurado.
package flow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Signal representa un evento dirigido a un flow_run específico.
type Signal struct {
	ID         int64           `json:"id"`
	FlowRunID  uuid.UUID       `json:"flow_run_id"`
	StepKey    *string         `json:"step_key,omitempty"`
	Name       string          `json:"name"`          // ej: "approve", "cancel", "input_received"
	Payload    json.RawMessage `json:"payload,omitempty"`
	DeliveredAt *time.Time     `json:"delivered_at,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

// SignalStore gestiona signals.
type SignalStore struct {
	Pool *pgxpool.Pool
}

var ErrSignalNotFound = errors.New("signal not found")

// SignalChannel es el canal pg NOTIFY usado para despertar waiters (sig-005).
const SignalChannel = "flow_signals"

// Send registra un signal nuevo. Idempotencia: si name='approve' ya existe
// y no-delivered para este run+step, se actualiza el payload en lugar de duplicar.
// Emite NOTIFY flow_signals con el run_id para despertar waiters sin polling.
func (s *SignalStore) Send(ctx context.Context, flowRunID uuid.UUID, stepKey *string, name string, payload []byte) (*Signal, error) {
	var sig Signal
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO flow_signals (flow_run_id, step_key, name, payload)
		VALUES ($1, $2, $3, $4)
		RETURNING id, flow_run_id, step_key, name, payload, delivered_at, created_at`,
		flowRunID, stepKey, name, payload,
	).Scan(&sig.ID, &sig.FlowRunID, &sig.StepKey, &sig.Name, &sig.Payload,
		&sig.DeliveredAt, &sig.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert signal: %w", err)
	}
	s.notify(ctx, flowRunID)
	return &sig, nil
}

// notify emite pg_notify(flow_signals, run_id). Best-effort: un fallo de
// NOTIFY no invalida el signal ya persistido (los waiters tienen fallback poll).
func (s *SignalStore) notify(ctx context.Context, flowRunID uuid.UUID) {
	_, _ = s.Pool.Exec(ctx, `SELECT pg_notify($1, $2)`, SignalChannel, flowRunID.String())
}

// HasPendingExpectation indica si el run espera (no expirado) la señal name.
func (s *SignalStore) HasPendingExpectation(ctx context.Context, runID uuid.UUID, name string) (bool, error) {
	var exists bool
	err := s.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM flow_run_signals_pending
			WHERE flow_run_id = $1 AND signal_name = $2 AND expires_at > NOW()
		)`, runID, name,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check pending: %w", err)
	}
	return exists, nil
}

// Consume devuelve el próximo signal no-delivered para (flowRun, stepKey, name)
// marcándolo como delivered en la misma tx. Atómico (SELECT FOR UPDATE SKIP LOCKED).
func (s *SignalStore) Consume(ctx context.Context, flowRunID uuid.UUID, stepKey *string, name string) (*Signal, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var sig Signal
	query := `
		SELECT id, flow_run_id, step_key, name, payload, delivered_at, created_at
		FROM flow_signals
		WHERE flow_run_id = $1
		  AND name = $2
		  AND delivered_at IS NULL
		  AND ($3::TEXT IS NULL OR step_key IS NULL OR step_key = $3)
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED`
	err = tx.QueryRow(ctx, query, flowRunID, name, stepKey).Scan(
		&sig.ID, &sig.FlowRunID, &sig.StepKey, &sig.Name, &sig.Payload,
		&sig.DeliveredAt, &sig.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSignalNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE flow_signals SET delivered_at = now() WHERE id = $1`, sig.ID,
	); err != nil {
		return nil, fmt.Errorf("mark delivered: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	now := time.Now()
	sig.DeliveredAt = &now
	return &sig, nil
}

// Wait poll-loop a Consume con timeout. Útil para steps que bloquean.
func (s *SignalStore) Wait(ctx context.Context, flowRunID uuid.UUID, stepKey *string, name string, timeout time.Duration, pollInterval time.Duration) (*Signal, error) {
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}
	deadline := time.Now().Add(timeout)
	for {
		sig, err := s.Consume(ctx, flowRunID, stepKey, name)
		if err == nil {
			return sig, nil
		}
		if !errors.Is(err, ErrSignalNotFound) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, ErrSignalNotFound
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// ErrSignalTimeout se retorna cuando WaitForSignal agota el timeout.
var ErrSignalTimeout = errors.New("signal timeout")

// SignalPendingExpectation registra que un flow_run step espera una señal externa.
type SignalPendingExpectation struct {
	ID         uuid.UUID `json:"id"`
	FlowRunID  uuid.UUID `json:"flow_run_id"`
	StepID     string    `json:"step_id"`
	SignalName string    `json:"signal_name"`
	ExpiresAt  time.Time `json:"expires_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// ExpectSignal registra la expectativa de señal para un step.
func (s *SignalStore) ExpectSignal(ctx context.Context, runID uuid.UUID, stepID, signalName string, ttl time.Duration) (*SignalPendingExpectation, error) {
	expiresAt := time.Now().Add(ttl)
	var exp SignalPendingExpectation
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO flow_run_signals_pending (flow_run_id, step_id, signal_name, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (flow_run_id, step_id) DO UPDATE SET
		  signal_name = EXCLUDED.signal_name,
		  expires_at = EXCLUDED.expires_at
		RETURNING id, flow_run_id, step_id, signal_name, expires_at, created_at`,
		runID, stepID, signalName, expiresAt,
	).Scan(&exp.ID, &exp.FlowRunID, &exp.StepID, &exp.SignalName, &exp.ExpiresAt, &exp.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert pending: %w", err)
	}
	return &exp, nil
}

// CancelExpectation elimina las expectativas de un run (run completed/cancelled).
func (s *SignalStore) CancelExpectation(ctx context.Context, runID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx,
		`DELETE FROM flow_run_signals_pending WHERE flow_run_id = $1`, runID)
	if err != nil {
		return fmt.Errorf("cancel expectation: %w", err)
	}
	return nil
}

// BroadcastSignal envía una señal con el nombre dado a TODOS los runs que la
// estén esperando. Retorna la cantidad de runs notificados.
func (s *SignalStore) BroadcastSignal(ctx context.Context, name string, payload []byte) (int, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT frp.flow_run_id, frp.step_id
		FROM flow_run_signals_pending frp
		WHERE frp.signal_name = $1
		  AND frp.expires_at > NOW()`,
		name,
	)
	if err != nil {
		return 0, fmt.Errorf("query pending: %w", err)
	}
	defer rows.Close()

	type target struct {
		runID  uuid.UUID
		stepID string
	}
	var targets []target
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.runID, &t.stepID); err != nil {
			return 0, fmt.Errorf("scan: %w", err)
		}
		targets = append(targets, t)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	for _, t := range targets {
		stepKey := t.stepID
		if _, err := s.Pool.Exec(ctx, `
			INSERT INTO flow_signals (flow_run_id, step_key, name, payload)
			VALUES ($1, $2, $3, $4)`,
			t.runID, stepKey, name, payload,
		); err != nil {
			return 0, fmt.Errorf("insert signal for run %s: %w", t.runID, err)
		}
		s.notify(ctx, t.runID)
	}

	return len(targets), nil
}

// WaitNotify espera la señal usando LISTEN/NOTIFY (sig-005) con fallback a
// polling si la conexión dedicada no puede establecerse. Sin CPU mientras
// espera: bloquea en WaitForNotification hasta NOTIFY o timeout.
func (s *SignalStore) WaitNotify(ctx context.Context, runID uuid.UUID, stepKey *string, name string, timeout time.Duration) (*Signal, error) {
	// Intento inmediato: la señal pudo llegar antes del LISTEN (early signal).
	sig, err := s.Consume(ctx, runID, stepKey, name)
	if err == nil {
		return sig, nil
	}
	if !errors.Is(err, ErrSignalNotFound) {
		return nil, err
	}

	conn, err := s.Pool.Acquire(ctx)
	if err != nil {
		return s.WaitForSignal(ctx, runID, stepKey, name, timeout)
	}
	// La conexión queda con estado LISTEN: cerrarla al salir para que el pool
	// no la reuse con suscripciones colgadas.
	defer func() {
		_ = conn.Conn().Close(context.Background())
		conn.Release()
	}()

	if _, err := conn.Exec(ctx, "LISTEN "+SignalChannel); err != nil {
		return s.WaitForSignal(ctx, runID, stepKey, name, timeout)
	}

	// Re-chequear post-LISTEN: cierra la ventana entre Consume y LISTEN.
	sig, err = s.Consume(ctx, runID, stepKey, name)
	if err == nil {
		return sig, nil
	}
	if !errors.Is(err, ErrSignalNotFound) {
		return nil, err
	}

	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, ErrSignalTimeout
		}
		nctx, cancel := context.WithTimeout(ctx, remaining)
		_, werr := conn.Conn().WaitForNotification(nctx)
		cancel()
		if werr != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if errors.Is(werr, context.DeadlineExceeded) {
				return nil, ErrSignalTimeout
			}
			return nil, fmt.Errorf("wait notification: %w", werr)
		}
		// Cualquier NOTIFY del canal: intentar consumir nuestra señal.
		sig, err := s.Consume(ctx, runID, stepKey, name)
		if err == nil {
			return sig, nil
		}
		if !errors.Is(err, ErrSignalNotFound) {
			return nil, err
		}
	}
}

// WaitForSignal es un wrapper sobre Wait que traduce ErrSignalNotFound a
// ErrSignalTimeout para integrar con retry policy (issue-09.4).
func (s *SignalStore) WaitForSignal(ctx context.Context, runID uuid.UUID, stepKey *string, name string, timeout time.Duration) (*Signal, error) {
	sig, err := s.Wait(ctx, runID, stepKey, name, timeout, 500*time.Millisecond)
	if errors.Is(err, ErrSignalNotFound) {
		return nil, ErrSignalTimeout
	}
	return sig, nil
}

// RemoveExpiredExpectations limpia expectativas vencidas.
func (s *SignalStore) RemoveExpiredExpectations(ctx context.Context) (int64, error) {
	tag, err := s.Pool.Exec(ctx,
		`DELETE FROM flow_run_signals_pending WHERE expires_at < NOW()`)
	if err != nil {
		return 0, fmt.Errorf("remove expired: %w", err)
	}
	return tag.RowsAffected(), nil
}

// List devuelve signals (delivered o pending) para un flow_run.
func (s *SignalStore) List(ctx context.Context, flowRunID uuid.UUID, includeDelivered bool) ([]Signal, error) {
	q := `SELECT id, flow_run_id, step_key, name, payload, delivered_at, created_at
	      FROM flow_signals WHERE flow_run_id = $1`
	if !includeDelivered {
		q += ` AND delivered_at IS NULL`
	}
	q += ` ORDER BY created_at ASC`
	rows, err := s.Pool.Query(ctx, q, flowRunID)
	if err != nil {
		return nil, fmt.Errorf("list signals: %w", err)
	}
	defer rows.Close()

	var out []Signal
	for rows.Next() {
		var sig Signal
		if err := rows.Scan(&sig.ID, &sig.FlowRunID, &sig.StepKey, &sig.Name,
			&sig.Payload, &sig.DeliveredAt, &sig.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, sig)
	}
	return out, rows.Err()
}
