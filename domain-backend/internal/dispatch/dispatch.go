package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// ErrUnknownTargetType retorna el dispatcher cuando TargetType no es
// reconocido. Se exporta para que los call-sites puedan hacer
// errors.Is() y reportar con HTTP status correcto.
var ErrUnknownTargetType = &UnknownTargetTypeError{}

// UnknownTargetTypeError error type para target_type desconocido.
type UnknownTargetTypeError struct {
	TargetType string
}

func (e *UnknownTargetTypeError) Error() string {
	return "unknown target_type: " + e.TargetType
}

// Dispatch ejecuta el target según req.TargetType, llamando al runner
// correspondiente. Centraliza:
//
//   - Selección de runner (switch único sobre target_type).
//   - Validación de source (loggea warning si es desconocido, no falla).
//   - Métricas unificadas (DispatchTotal + DispatchDuration).
//   - Audit pre/post (dispatch.started + dispatch.completed).
//
// Si req.TargetType no es reconocido → ErrUnknownTargetType.
// Si el runner retorna error → bubbled up, métricas con result=failed,
// audit con error en metadata.
func (d *Dispatcher) Dispatch(ctx context.Context, req Request) (Result, error) {
	start := time.Now()
	logger := d.logger()

	// Source validation: warning si es desconocido, no falla.
	if d.SourceValidator != nil && !d.SourceValidator(req.Source) {
		logger.Warn("unknown source, dispatching anyway",
			slog.String("source", req.Source),
			slog.String("target_type", req.TargetType))
	}

	// Audit pre.
	if d.Audit != nil {
		_ = d.Audit.Record(ctx, AuditEvent{
			OrgID:      req.OrgID,
			EntityType: "dispatch",
			EntityID:   req.TargetID,
			Action:     "dispatch.started",
			Metadata: map[string]any{
				"source":      req.Source,
				"target_type": req.TargetType,
			},
		})
	}

	// Switch único.
	var (
		result Result
		err    error
	)
	switch req.TargetType {
	case TargetFlow:
		if d.RunFlow == nil {
			err = errors.New("dispatcher: flow runner not configured")
		} else {
			result, err = d.RunFlow(ctx, req)
		}
	case TargetAgent:
		if d.RunAgent == nil {
			err = errors.New("dispatcher: agent runner not configured")
		} else {
			result, err = d.RunAgent(ctx, req)
		}
	case TargetSkill:
		if d.RunSkill == nil {
			err = errors.New("dispatcher: skill runner not configured")
		} else {
			result, err = d.RunSkill(ctx, req)
		}
	default:
		err = fmt.Errorf("%w: %s", ErrUnknownTargetType, req.TargetType)
	}

	// Result label.
	resultLabel := "success"
	if err != nil {
		resultLabel = "failed"
	}

	// Métricas.
	if d.Metrics != nil {
		d.Metrics.ObserveDispatch(req.Source, req.TargetType, resultLabel,
			time.Since(start).Seconds())
	}

	// Audit post.
	if d.Audit != nil {
		meta := map[string]any{
			"source":      req.Source,
			"target_type": req.TargetType,
			"result":      resultLabel,
			"duration_ms": time.Since(start).Milliseconds(),
		}
		if err != nil {
			meta["error"] = err.Error()
		}
		_ = d.Audit.Record(ctx, AuditEvent{
			OrgID:      req.OrgID,
			EntityType: "dispatch",
			EntityID:   req.TargetID,
			Action:     "dispatch.completed",
			Metadata:   meta,
		})
	}

	return result, err
}

func (d *Dispatcher) logger() *slog.Logger {
	if d.Logger != nil {
		return d.Logger
	}
	return slog.Default()
}

// Compile-time check: json.RawMessage es el tipo de Inputs.
var _ json.RawMessage
