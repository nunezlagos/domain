// Package cronsched — HU-10.1 scheduler loop que poll-pickea crons due.
//
// Multi-worker safe (PickDue usa SELECT FOR UPDATE SKIP LOCKED).
// Stop signal vía context cancel para graceful shutdown.
package cronsched

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/audit"
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

		var execErr error
		switch c.TargetType {
		case "flow":
			if s.Flows == nil {
				execErr = fmt.Errorf("flow runner not configured")
				break
			}
			_, execErr = s.Flows.Run(dispatchCtx, flowrunner.RunInput{
				FlowID: c.TargetID, TriggerType: "cron", Inputs: c.Inputs,
			})
		case "agent":
			if s.Agents == nil {
				execErr = fmt.Errorf("agent runner not configured")
				break
			}
			input, _ := c.Inputs["input"].(string)
			_, execErr = s.Agents.Run(dispatchCtx, agentrunner.RunInput{
				AgentID:    c.TargetID,
				UserPrompt: input,
				Variables:  c.Inputs,
			})
		case "skill":
			if s.Skills == nil || s.SkillRunner == nil {
				execErr = fmt.Errorf("skill runner not configured")
				break
			}
			sk, err := s.Skills.GetByID(dispatchCtx, c.TargetID)
			if err != nil {
				execErr = fmt.Errorf("load skill: %w", err)
				break
			}
			_, execErr = s.SkillRunner.Execute(dispatchCtx, sk, c.Inputs)
		default:
			execErr = fmt.Errorf("unknown target_type: %s", c.TargetType)
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

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
