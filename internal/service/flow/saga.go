// HU-09.9 saga-compensation — patrón Saga para flows con side-effects
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
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SagaCompensation describe la action que revierte un step exitoso.
type SagaCompensation struct {
	StepKey       string          `json:"step_key"`
	CompensateKey string          `json:"compensate_key"`  // referencia a otro step
	InputMapping  json.RawMessage `json:"input_mapping,omitempty"`
	MaxRetries    int             `json:"max_retries,omitempty"`
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

// SagaExecutor ejecuta compensaciones en orden inverso al fallo de un step.
type SagaExecutor struct {
	Pool   *pgxpool.Pool
	Logger *slog.Logger
	// RunCompensate es la función que ejecuta el step compensatorio.
	// Provista por el runner del flow (no podemos invocar tipos de runner
	// aquí para evitar ciclo de imports).
	RunCompensate func(ctx context.Context, runID uuid.UUID, stepKey string, input json.RawMessage) error
}

// ExecuteCompensations corre las compensaciones en orden inverso de
// completedSteps (lista ordered ascending de step_keys que sí terminaron OK).
func (s *SagaExecutor) ExecuteCompensations(ctx context.Context, runID uuid.UUID, completedSteps []string, plan []SagaCompensation) error {
	if s.Logger == nil {
		s.Logger = slog.Default()
	}
	planByStep := map[string]SagaCompensation{}
	for _, p := range plan {
		planByStep[p.StepKey] = p
	}

	for i := len(completedSteps) - 1; i >= 0; i-- {
		stepKey := completedSteps[i]
		comp, ok := planByStep[stepKey]
		if !ok {
			continue // step sin compensación declarada — se asume idempotente o sin side-effect
		}

		s.Logger.Info("running saga compensation",
			slog.String("run_id", runID.String()),
			slog.String("original_step", stepKey),
			slog.String("compensate", comp.CompensateKey),
		)

		err := s.runCompensateWithRetry(ctx, runID, comp)
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		s.logCompensation(ctx, runID, stepKey, comp.CompensateKey, err == nil, errMsg, comp.InputMapping)

		if err != nil {
			s.Logger.Error("saga compensation failed",
				slog.String("run_id", runID.String()),
				slog.String("step", stepKey),
				slog.Any("err", err),
			)
			// Continuar con las otras compensaciones aun si una falla — best effort.
		}
	}
	return nil
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
		// Backoff exponencial leve
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 500 * time.Millisecond):
		}
	}
	return lastErr
}

func (s *SagaExecutor) logCompensation(ctx context.Context, runID uuid.UUID, origStep, compRan string, success bool, errMsg string, payload json.RawMessage) {
	var em *string
	if errMsg != "" {
		em = &errMsg
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO saga_compensation_log
		  (run_id, original_step, compensate_ran, success, error, payload)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		runID, origStep, compRan, success, em, payload,
	)
	if err != nil {
		s.Logger.Warn("failed to log compensation",
			slog.String("run_id", runID.String()), slog.Any("err", err))
	}
}
