// Package cronsched — issue-10.1 scheduler loop que poll-pickea crons due.
//
// Multi-worker safe (PickDue usa SELECT FOR UPDATE SKIP LOCKED).
// Stop signal vía context cancel para graceful shutdown.
//
// issue-35.1: el switch de dispatchSync fue reemplazado por una
// llamada a internal/dispatch.Dispatcher. Mantenemos los campos
// legacy (Agents/Flows/SkillRunner/Skills) por compat: el boot puede
// inyectar el dispatcher + los runners por separado, o solo el
// dispatcher. Si Dispatcher == nil, dispatchSync cae al switch
// legacy (mantiene los tests scheduler_test.go funcionando hasta
// que se haga la limpieza final — phase 5 de REQ-35.1).
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
	agentrunner "nunezlagos/domain/internal/runner/agent"
	flowrunner "nunezlagos/domain/internal/runner/flow"
	skillrunner "nunezlagos/domain/internal/runner/skill"
	"nunezlagos/domain/internal/service/cron"
	skillsvc "nunezlagos/domain/internal/service/skill"
)

type Scheduler struct {
	Crons        *cron.Service
	Agents       *agentrunner.Runner
	Flows        *flowrunner.Runner
	SkillRunner  *skillrunner.Runner
	Skills       *skillsvc.Service
	Audit        audit.Recorder
	Logger       *slog.Logger
	PollInterval time.Duration // default 30s
	// Dispatcher (issue-35.1): si no nil, dispatchSync delega acá.
	// Si nil, usa el switch legacy (compat con tests existentes).
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

		execErr := s.dispatchSync(dispatchCtx, c)

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

// dispatchSync ejecuta el target del cron y devuelve el error.
// Separado de dispatch() para testing sincrónico.
//
// issue-35.1: si s.Dispatcher está configurado, delega al dispatcher
// unificado (métricas + audit centralizados). Si no, usa el switch
// legacy (compat con tests existentes que no setean Dispatcher).
func (s *Scheduler) dispatchSync(ctx context.Context, c cron.Cron) error {
	if s.Dispatcher != nil {
		// Serializar inputs a json.RawMessage.
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

	// Legacy switch (mantenido hasta phase 5 de REQ-35.1).
	switch c.TargetType {
	case "flow":
		if s.Flows == nil {
			return fmt.Errorf("flow runner not configured")
		}
		_, err := s.Flows.Run(ctx, flowrunner.RunInput{
			FlowID: c.TargetID, TriggerType: "cron", Inputs: c.Inputs,
		})
		return err
	case "agent":
		if s.Agents == nil {
			return fmt.Errorf("agent runner not configured")
		}
		input, _ := c.Inputs["input"].(string)
		_, err := s.Agents.Run(ctx, agentrunner.RunInput{
			AgentID: c.TargetID, UserPrompt: input, Variables: c.Inputs,
		})
		return err
	case "skill":
		if s.Skills == nil || s.SkillRunner == nil {
			return fmt.Errorf("skill runner not configured")
		}
		sk, err := s.Skills.GetByID(ctx, c.TargetID)
		if err != nil {
			return fmt.Errorf("load skill: %w", err)
		}
		_, err = s.SkillRunner.Execute(ctx, sk, c.Inputs)
		return err
	default:
		return fmt.Errorf("unknown target_type: %s", c.TargetType)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
