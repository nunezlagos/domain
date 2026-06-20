// Package cronsched — issue-10.1 scheduler loop que poll-pickea crons due.
//
// Multi-worker safe (PickDue usa SELECT FOR UPDATE SKIP LOCKED).
// Stop signal vía context cancel para graceful shutdown.
//
// issue-35.1 (phase 5): el switch local de target_type→runner fue
// eliminado. Ahora runTarget delega EXCLUSIVAMENTE a
// internal/dispatch.Dispatcher. La selección de runner vive en 1
// solo lugar (el dispatcher), evitando que un nuevo target_type se
// olvide en alguno de los 3 sources (cron, webhook, mcp).
package cronsched

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/dispatch"
	"nunezlagos/domain/internal/service/cron"
)

type Scheduler struct {
	Crons        *cron.Service
	Audit        audit.Recorder
	Logger       *slog.Logger
	PollInterval time.Duration // default 30s
	// Dispatcher (issue-35.1): único path para ejecutar target_type.
	// REQUERIDO: si es nil, runTarget retorna error.
	Dispatcher *dispatch.Dispatcher
}

// Run inicia el loop. Bloquea hasta que ctx se cancele.
func (s *Scheduler) Run(ctx context.Context) {
	interval := s.PollInterval
	if interval == 0 {
		interval = 30 * time.Second
	}
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}

	logger.Info("cron scheduler started", slog.Duration("poll", interval))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("cron scheduler stopping")
			return
		case <-ticker.C:
			s.tick(ctx, logger)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context, logger *slog.Logger) {
	due, err := s.Crons.PickDue(ctx, 50)
	if err != nil {
		logger.Error("pick due crons failed", slog.Any("err", err))
		return
	}
	if len(due) == 0 {
		return
	}
	logger.Info("crons due", slog.Int("count", len(due)))
	for _, c := range due {
		s.dispatch(ctx, c, logger)
	}
}

func (s *Scheduler) dispatch(ctx context.Context, c cron.Cron, logger *slog.Logger) {
	logger.Info("dispatch cron",
		slog.String("slug", c.Slug),
		slog.String("target_type", c.TargetType),
		slog.String("target_id", c.TargetID.String()))

	// Ejecutar en goroutine para no bloquear el scheduler si hay muchos
	go func() {
		dispatchCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		execID, skipped, histErr := s.Crons.StartExecution(dispatchCtx, c.ID, c.TargetType)
		if histErr != nil {
			logger.Error("cron history start failed",
				slog.String("slug", c.Slug), slog.Any("err", histErr))
		}
		if skipped {
			logger.Warn("cron overlap: previous execution still running, skipping",
				slog.String("slug", c.Slug))
			return
		}

		execErr := s.runTarget(dispatchCtx, c)

		if histErr == nil {
			if err := s.Crons.FinishExecution(dispatchCtx, execID, execErr); err != nil {
				logger.Error("cron history finish failed",
					slog.String("slug", c.Slug), slog.Any("err", err))
			}
		}
		if execErr != nil {
			logger.Error("cron exec failed",
				slog.String("slug", c.Slug), slog.Any("err", execErr))
		}
		if s.Audit != nil {
			action := "cron.executed"
			if execErr != nil {
				action = "cron.failed"
			}
			cid := c.ID
			_ = s.Audit.Record(dispatchCtx, audit.Event{
				OrganizationID: &c.OrganizationID,
				ActorType:      audit.ActorSystem,
				Action:         action,
				EntityType:     "cron",
				EntityID:       &cid,
				NewValues: map[string]any{
					"target_type": c.TargetType,
					"error":       errString(execErr),
				},
			})
		}
		_ = uuid.Nil // keep import
	}()
}

// runTarget ejecuta el target del cron delegando al dispatcher unificado.
// Si el Dispatcher no fue configurado al boot, retorna error explícito
// (no hay path legacy: phase 5 de REQ-35.1 eliminó el switch local).
func (s *Scheduler) runTarget(ctx context.Context, c cron.Cron) error {
	if s.Dispatcher == nil {
		return fmt.Errorf("cron scheduler: dispatcher not configured")
	}
	var inputsRaw json.RawMessage
	if c.Inputs != nil {
		b, err := json.Marshal(c.Inputs)
		if err != nil {
			return fmt.Errorf("marshal cron inputs: %w", err)
		}
		inputsRaw = b
	}
	_, err := s.Dispatcher.Dispatch(ctx, dispatch.Request{
		OrgID:      c.OrganizationID,
		Source:     dispatch.SourceCron,
		TargetType: c.TargetType,
		TargetID:   c.TargetID,
		Inputs:     inputsRaw,
	})
	return err
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
