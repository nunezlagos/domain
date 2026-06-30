// issue-09.4 — retry policies por step (exponential/fixed/retry_on),
// fallback chain y push a Dead Letter Queue en fallos permanentes.
package flowrunner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/flow"
)

// classifyError mapea un error a un tipo machine-readable para retry_on.
func classifyError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline"):
		return "timeout"
	case strings.Contains(msg, "rate limit") || strings.Contains(msg, "429") || strings.Contains(msg, "too many requests"):
		return "rate_limit"
	case strings.Contains(msg, "connection") || strings.Contains(msg, "no such host") || strings.Contains(msg, "eof"):
		return "network"
	case strings.Contains(msg, "validation") || strings.Contains(msg, "invalid") || strings.Contains(msg, "required"):
		return "validation_error"
	case strings.Contains(msg, "not found"):
		return "not_found"
	default:
		return "unknown"
	}
}

// retryPlan resuelve la política efectiva del step (rica con precedencia
// sobre legacy Retries/MaxBackoffS).
type retryPlan struct {
	maxAttempts int
	retryOn     []string // vacío = todos
	rich        bool     // true si viene de step.Retry (no aplica gate isTransient)
	delay       func(attempt int) time.Duration
}

func buildRetryPlan(step *flow.Step) retryPlan {
	if rp := step.Retry; rp != nil {
		maxRetries := rp.MaxRetries
		if maxRetries > flow.MaxRetriesCap {
			maxRetries = flow.MaxRetriesCap
		}
		plan := retryPlan{maxAttempts: maxRetries + 1, retryOn: rp.RetryOn, rich: true}
		switch rp.Backoff {
		case "fixed":
			d := time.Duration(rp.FixedDelayMs) * time.Millisecond
			if d <= 0 {
				d = time.Second
			}
			plan.delay = func(int) time.Duration { return d }
		default: // exponential
			initial := time.Duration(rp.InitialDelayMs) * time.Millisecond
			if initial <= 0 {
				initial = 200 * time.Millisecond
			}
			maxBackoff := 30 * time.Second
			if step.MaxBackoffS > 0 {
				maxBackoff = time.Duration(step.MaxBackoffS) * time.Second
			}
			plan.delay = func(attempt int) time.Duration {
				d := initial << (attempt - 1)
				if d > maxBackoff {
					d = maxBackoff
				}
				return d
			}
		}
		return plan
	}


	maxBackoff := 30 * time.Second
	if step.MaxBackoffS > 0 {
		maxBackoff = time.Duration(step.MaxBackoffS) * time.Second
	}
	return retryPlan{
		maxAttempts: step.Retries + 1,
		delay: func(attempt int) time.Duration {
			d := 200 * time.Millisecond << (attempt - 1)
			if d > maxBackoff {
				d = maxBackoff
			}
			return d
		},
	}
}

// shouldRetry decide si el error es reintentable bajo el plan.
func (p retryPlan) shouldRetry(err error) bool {
	if p.rich {
		if len(p.retryOn) == 0 {
			return true // retry_on vacío = todos los errores (escenario 3)
		}
		kind := classifyError(err)
		for _, k := range p.retryOn {
			if k == kind {
				return true
			}
		}
		return false
	}
	return isTransientError(err)
}

// runStepWithRetry ejecuta el step aplicando su retry policy.
// Retorna el output, el error final (nil si OK), los mensajes de cada
// intento fallido y la cantidad de reintentos efectuados.
func (r *Runner) runStepWithRetry(ctx context.Context, runID uuid.UUID, step *flow.Step,
	inputs, outputs map[string]any, orgID uuid.UUID, userID *uuid.UUID,
	flowSlug string) (out any, stepErr error, attemptErrors []string, retryCount int) {

	plan := buildRetryPlan(step)
	for attempt := 1; attempt <= plan.maxAttempts; attempt++ {
		out, stepErr = r.executeStep(ctx, runID, step, inputs, outputs, orgID, userID)
		if stepErr == nil {
			return out, nil, attemptErrors, retryCount
		}
		attemptErrors = append(attemptErrors, stepErr.Error())
		if !plan.shouldRetry(stepErr) || attempt == plan.maxAttempts {
			return out, stepErr, attemptErrors, retryCount
		}
		retryCount++
		if r.Metrics != nil {
			r.Metrics.FlowStepRetriesTotal.WithLabelValues(flowSlug, step.ID).Inc()
		}

		delay := plan.delay(attempt)
		dt := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			dt.Stop()
			return out, ctx.Err(), append(attemptErrors, ctx.Err().Error()), retryCount
		case <-dt.C:
		}
	}
	return out, stepErr, attemptErrors, retryCount
}

// execFallback ejecuta la cadena de fallback_steps (máximo flow.MaxFallbackDepth).
// Si un fallback falla, aplica SU política recursivamente; sin política → abort.
func (r *Runner) execFallback(ctx context.Context, runID uuid.UUID, failed *flow.Step,
	inputs, outputs map[string]any, orgID uuid.UUID, userID *uuid.UUID,
	flowSlug string, depth int) (any, error) {

	fb := failed.FallbackStep
	if fb == nil {
		return nil, fmt.Errorf("step '%s': fallback_step not defined", failed.ID)
	}
	if depth > flow.MaxFallbackDepth {
		return nil, fmt.Errorf("step '%s': fallback chain exceeds %d levels", failed.ID, flow.MaxFallbackDepth)
	}

	out, err, _, _ := r.runStepWithRetry(ctx, runID, fb, inputs, outputs, orgID, userID, flowSlug)
	if err == nil {
		return out, nil
	}

	switch fb.OnError {
	case "continue", "ignore_and_continue":
		if fb.DefaultOnError != nil {
			return fb.DefaultOnError, nil
		}
		return map[string]any{"error": err.Error()}, nil
	case "fallback_step":
		return r.execFallback(ctx, runID, fb, inputs, outputs, orgID, userID, flowSlug, depth+1)
	default:
		return nil, fmt.Errorf("fallback '%s' failed: %w", fb.ID, err)
	}
}
