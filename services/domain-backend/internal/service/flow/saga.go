// issue-09.9 saga-compensation — patrón Saga para flows con side-effects
// externos (envío email, write DB, llamada API). Cada step puede declarar
// un compensation step que revierte su efecto si un step posterior falla.
//
// Ejemplo flow Saga:
//
//	step 1: reservar_inventario      → compensar: liberar_inventario
//	step 2: cargar_tarjeta           → compensar: refund_tarjeta
//	step 3: enviar_confirmacion      → compensar: enviar_cancelacion
//
// Si step 3 falla, el executor ejecuta compensaciones en orden inverso:
// compensar step 2, luego compensar step 1.
package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RetryPolicy define cómo se reintenta una compensación fallida.
type RetryPolicy string

const (
	RetryIdempotent     RetryPolicy = "idempotent"      // safe to retry, standard backoff
	RetryReEmit         RetryPolicy = "re-emit"         // must re-emit the compensation action
	RetryRequireCleanup RetryPolicy = "require-cleanup" // requires manual cleanup before retry
)

// SagaCompensation describe la action que revierte un step exitoso.
type SagaCompensation struct {
	StepKey       string          `json:"step_key"`
	CompensateKey string          `json:"compensate_key"` // referencia a otro step
	InputMapping  json.RawMessage `json:"input_mapping,omitempty"`
	MaxRetries    int             `json:"max_retries,omitempty"`
	RetryPolicy   RetryPolicy     `json:"retry_policy,omitempty"`
}

// CompensationLog registra ejecuciones de compensación.
type CompensationLog struct {
	ID            int64           `json:"id"`
	RunID         uuid.UUID       `json:"run_id"`
	OriginalStep  string          `json:"original_step"`
	CompensateRan string          `json:"compensate_ran"`
	Success       bool            `json:"success"`
	Error         *string         `json:"error,omitempty"`
	Payload       json.RawMessage `json:"payload,omitempty"`
	ExecutedAt    time.Time       `json:"executed_at"`
}

// CompensationFailure registra una compensación que falló incluso tras retries.
// Estos registros requieren intervención manual (issue-09.9 escenario 2).
type CompensationFailure struct {
	ID            int64       `json:"id"`
	RunID         uuid.UUID   `json:"run_id"`
	OriginalStep  string      `json:"original_step"`
	CompensateRan string      `json:"compensate_ran"`
	Error         string      `json:"error"`
	RetryPolicy   RetryPolicy `json:"retry_policy"`
	Attempts      int         `json:"attempts"`
	Skipped       bool        `json:"skipped"`
	SkippedReason *string     `json:"skipped_reason,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
}

// SagaStore gestiona el registro, ejecución y consulta de compensaciones.
// Mantiene un registro en-memoria de compensaciones pendientes por run.
// Para resiliencia ante crash, las compensaciones se reconstruyen del spec
// del flow al reanudar.
type SagaStore struct {
	Pool *pgxpool.Pool

	mu     sync.Mutex
	plans  map[uuid.UUID][]SagaCompensation // runID → compensaciones registradas
	logger *slog.Logger
}

func NewSagaStore(pool *pgxpool.Pool) *SagaStore {
	return &SagaStore{
		Pool:   pool,
		plans:  make(map[uuid.UUID][]SagaCompensation),
		logger: slog.Default(),
	}
}

// RegisterCompensation registra una compensación para un step exitoso.
// Si el step existe en los planes del run, se reemplaza (idempotente).
func (s *SagaStore) RegisterCompensation(ctx context.Context, runID uuid.UUID, comp SagaCompensation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Reemplazar si ya existe para este stepKey (idempotente)
	existing := s.plans[runID]
	found := false
	for i, c := range existing {
		if c.StepKey == comp.StepKey {
			existing[i] = comp
			found = true
			break
		}
	}
	if !found {
		s.plans[runID] = append(existing, comp)
	}

	s.logger.Debug("compensation registered",
		slog.String("run_id", runID.String()),
		slog.String("step_key", comp.StepKey),
		slog.String("compensate_key", comp.CompensateKey),
	)
	return nil
}

// ExecuteCompensation ejecuta la compensación de un step específico.
// Retorna error si la compensación no está registrada o falla.
func (s *SagaStore) ExecuteCompensation(ctx context.Context, runID uuid.UUID, stepKey string) error {
	comp, err := s.lookupCompensation(runID, stepKey)
	if err != nil {
		return err
	}

	err = s.runCompensateWithRetry(ctx, runID, comp)
	s.logCompensation(ctx, runID, comp.StepKey, comp.CompensateKey, err == nil, errMsg(err), comp.InputMapping)

	return err
}

func errMsg(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// lookupCompensation busca una compensación registrada para stepKey.
func (s *SagaStore) lookupCompensation(runID uuid.UUID, stepKey string) (SagaCompensation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, ok := s.plans[runID]
	if !ok {
		return SagaCompensation{}, fmt.Errorf("no compensations registered for run %s", runID)
	}
	for _, c := range entries {
		if c.StepKey == stepKey {
			return c, nil
		}
	}
	return SagaCompensation{}, fmt.Errorf("compensation not found for step %s in run %s", stepKey, runID)
}

// runCompensateWithRetry ejecuta la compensación con reintentos según retry policy.
func (s *SagaStore) runCompensateWithRetry(ctx context.Context, runID uuid.UUID, comp SagaCompensation) error {
	maxRetries := comp.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := s.execOneCompensation(ctx, runID, comp)
		if err == nil {
			return nil
		}
		lastErr = err

		// Si requiere cleanup manual, no reintentar automáticamente
		if comp.RetryPolicy == RetryRequireCleanup {
			break
		}

		// ISSUE-28.8: NewTimer reusable en loop (time.After leak).
		backoff := time.Duration(attempt+1) * 500 * time.Millisecond
		t := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
	return lastErr
}

// execOneCompensation ejecuta un intento de compensación.
// Si el RunCompensate hook no está configurado, se delega al pool.
func (s *SagaStore) execOneCompensation(ctx context.Context, runID uuid.UUID, comp SagaCompensation) error {
	_ = runID
	// La ejecución real está en el runner que provee RunCompensate
	return fmt.Errorf("RunCompensate hook not configured for step %s", comp.StepKey)
}

// logCompensation registra la ejecución de una compensación.
// REQ-42.3: saga_compensation_log dropeada — el log ya no se persiste en DB;
// queda como traza estructurada (passthrough en memoria/logger).
func (s *SagaStore) logCompensation(ctx context.Context, runID uuid.UUID, origStep, compRan string, success bool, errMsg string, payload json.RawMessage) {
	_ = ctx
	_ = payload
	s.logger.Debug("compensation logged",
		slog.String("run_id", runID.String()),
		slog.String("original_step", origStep),
		slog.String("compensate_ran", compRan),
		slog.Bool("success", success),
		slog.String("error", errMsg),
	)
}

// logCompensationFailure registra una compensación que falló tras retries.
// REQ-42.3: saga_compensation_log dropeada — sin persistencia en DB.
func (s *SagaStore) logCompensationFailure(ctx context.Context, runID uuid.UUID, comp SagaCompensation, errStr string, attempts int) {
	_ = ctx
	s.logger.Warn("compensation failed (manual cleanup may be required)",
		slog.String("run_id", runID.String()),
		slog.String("original_step", comp.StepKey),
		slog.String("compensate_ran", comp.CompensateKey),
		slog.String("error", errStr),
		slog.Int("attempts", attempts),
	)
}

// GetLog devuelve los logs de compensación para un run.
// REQ-42.3: saga_compensation_log dropeada — sin store persistente, devuelve vacío.
func (s *SagaStore) GetLog(ctx context.Context, runID uuid.UUID) ([]CompensationLog, error) {
	_ = ctx
	_ = runID
	return nil, nil
}

// RegisteredCompensations devuelve las compensaciones registradas para un run.
func (s *SagaStore) RegisteredCompensations(runID uuid.UUID) []SagaCompensation {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, ok := s.plans[runID]
	if !ok {
		return nil
	}
	out := make([]SagaCompensation, len(entries))
	copy(out, entries)
	return out
}

// ClearRun libera la memoria de las compensaciones de un run.
func (s *SagaStore) ClearRun(runID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.plans, runID)
}

var _ = pgx.ErrNoRows // uso indirecto

// SagaExecutor ejecuta compensaciones en orden inverso al fallo de un step.
type SagaExecutor struct {
	Pool   *pgxpool.Pool
	Logger *slog.Logger
	// Store es el almacén de compensaciones.
	Store *SagaStore
	// RunCompensate es la función que ejecuta el step compensatorio.
	// Provista por el runner del flow (no podemos invocar tipos de runner
	// aquí para evitar ciclo de imports).
	RunCompensate func(ctx context.Context, runID uuid.UUID, stepKey string, input json.RawMessage) error
	// CompensateInParallel habilita ejecución paralela de compensaciones.
	// Si algún step tiene RetryPolicy=require-cleanup, se ignora esta flag.
	CompensateInParallel bool
}

// ExecuteCompensations corre las compensaciones en orden inverso de
// completedSteps (lista ordered ascending de step_keys que sí terminaron OK).
// Soporta compensaciones en paralelo (CompensateInParallel flag).
func (s *SagaExecutor) ExecuteCompensations(ctx context.Context, runID uuid.UUID, completedSteps []string, plan []SagaCompensation) error {
	if s.Logger == nil {
		s.Logger = slog.Default()
	}
	planByStep := map[string]SagaCompensation{}
	for _, p := range plan {
		planByStep[p.StepKey] = p
	}

	// Identificar qué compensaciones ejecutar (solo steps que tienen compensate_ran distinto de vacío)
	var toExecute []SagaCompensation
	for i := len(completedSteps) - 1; i >= 0; i-- {
		stepKey := completedSteps[i]
		comp, ok := planByStep[stepKey]
		if !ok {
			continue
		}
		if comp.CompensateKey == "" {
			continue // compensate vacío → step idempotente
		}
		toExecute = append(toExecute, comp)
	}

	if len(toExecute) == 0 {
		s.Logger.Info("no compensations to execute",
			slog.String("run_id", runID.String()))
		return nil
	}

	// Si algún step requiere cleanup manual, ejecutar secuencial
	parallel := s.CompensateInParallel
	for _, comp := range toExecute {
		if comp.RetryPolicy == RetryRequireCleanup {
			parallel = false
			break
		}
	}

	if parallel && len(toExecute) > 1 {
		return s.executeCompensationsParallel(ctx, runID, toExecute)
	}
	return s.executeCompensationsSequential(ctx, runID, toExecute)
}

func (s *SagaExecutor) executeCompensationsSequential(ctx context.Context, runID uuid.UUID, compensations []SagaCompensation) error {
	for _, comp := range compensations {
		s.Logger.Info("running saga compensation",
			slog.String("run_id", runID.String()),
			slog.String("original_step", comp.StepKey),
			slog.String("compensate", comp.CompensateKey),
		)

		err := s.runCompensateWithRetry(ctx, runID, comp)
		s.logResult(ctx, runID, comp.StepKey, comp.CompensateKey, err, comp.InputMapping)

		if err != nil {
			s.Logger.Error("saga compensation failed",
				slog.String("run_id", runID.String()),
				slog.String("step", comp.StepKey),
				slog.Any("err", err),
			)
			// Best effort: continuar con las otras compensaciones
		}
	}
	return nil
}

func (s *SagaExecutor) executeCompensationsParallel(ctx context.Context, runID uuid.UUID, compensations []SagaCompensation) error {
	type result struct {
		comp SagaCompensation
		err  error
	}
	ch := make(chan result, len(compensations))

	for _, comp := range compensations {
		go func(c SagaCompensation) {
			s.Logger.Info("running saga compensation (parallel)",
				slog.String("run_id", runID.String()),
				slog.String("original_step", c.StepKey),
				slog.String("compensate", c.CompensateKey),
			)
			err := s.runCompensateWithRetry(ctx, runID, c)
			s.logResult(ctx, runID, c.StepKey, c.CompensateKey, err, c.InputMapping)
			if err != nil {
				s.Logger.Error("saga compensation failed (parallel)",
					slog.String("run_id", runID.String()),
					slog.String("step", c.StepKey),
					slog.Any("err", err),
				)
			}
			ch <- result{comp: c, err: err}
		}(comp)
	}

	var lastErr error
	for i := 0; i < len(compensations); i++ {
		r := <-ch
		if r.err != nil {
			lastErr = r.err
		}
	}
	return lastErr
}

// logResult loggea a DB si Pool está configurado, sino solo log a console.
func (s *SagaExecutor) logResult(ctx context.Context, runID uuid.UUID, origStep, compRan string, err error, payload json.RawMessage) {
	if s.Pool == nil {
		s.Logger.Info("compensation result (no db)",
			slog.String("run_id", runID.String()),
			slog.String("step", origStep),
			slog.Bool("success", err == nil),
		)
		return
	}
	s.logCompensation(ctx, runID, origStep, compRan, err == nil, errMsg(err), payload)
}

func (s *SagaExecutor) runCompensateWithRetry(ctx context.Context, runID uuid.UUID, comp SagaCompensation) error {
	maxRetries := comp.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if s.RunCompensate == nil {
			return fmt.Errorf("RunCompensate hook not configured")
		}
		err := s.RunCompensate(ctx, runID, comp.CompensateKey, comp.InputMapping)
		if err == nil {
			return nil
		}
		lastErr = err

		// Si requiere cleanup manual, no reintentar
		if comp.RetryPolicy == RetryRequireCleanup {
			break
		}

		// ISSUE-28.8: NewTimer reusable en loop (time.After leak).
		backoff := time.Duration(attempt+1) * 500 * time.Millisecond
		t := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
	return lastErr
}

// logCompensation registra el resultado de una compensación.
// REQ-42.3: saga_compensation_log dropeada — sin persistencia en DB.
func (s *SagaExecutor) logCompensation(ctx context.Context, runID uuid.UUID, origStep, compRan string, success bool, errMsg string, payload json.RawMessage) {
	_ = ctx
	_ = payload
	s.Logger.Debug("compensation logged",
		slog.String("run_id", runID.String()),
		slog.String("original_step", origStep),
		slog.String("compensate_ran", compRan),
		slog.Bool("success", success),
		slog.String("error", errMsg),
	)
}
