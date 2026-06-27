package main

import (
	"context"
	"log/slog"
	"time"

	"nunezlagos/domain/internal/config"
	"nunezlagos/domain/internal/metrics"
	flowrunner "nunezlagos/domain/internal/runner/flow"
	cronsched "nunezlagos/domain/internal/scheduler/cron"
	systemcron "nunezlagos/domain/internal/scheduler/cron/system"
	"nunezlagos/domain/internal/scheduler/leader"
)

// serverRunners agrupa scheduler, leaderElection y su contexto de cancelación.
type serverRunners struct {
	Scheduler      *cronsched.Scheduler
	LeaderElection *leader.Election
	SchedCancel    context.CancelFunc
}

// buildRunners construye el scheduler y el leader election, y arranca las
// goroutines de background que corren bajo el lock de leader.
// El caller debe diferir runners.SchedCancel() para parar el scheduler al shutdown.
func buildRunners(
	ctx context.Context,
	cfg *config.Config,
	pools serverPools,
	s *serverServices,
	metricsReg *metrics.Registry,
	logger *slog.Logger,
) serverRunners {
	scheduler := &cronsched.Scheduler{
		Crons:      s.CronService,
		Audit:      s.Recorder,
		Logger:     logger,
		Dispatcher: s.Dispatcher,
	}
	leaderElection := &leader.Election{
		Pool:       pools.App,
		LockKey:    leader.LockKeyCronScheduler,
		PollPeriod: 10 * time.Second,
		Logger:     logger,
	}

	schedCtx, schedCancel := context.WithCancel(context.Background())
	go leaderElection.RunAsLeader(schedCtx, func(leaderCtx context.Context) {
		go s.FlowRunnerInst.RunRecovery(leaderCtx, flowrunner.RecoveryConfig{
			StaleAfter: 5 * time.Minute, PollInterval: 60 * time.Second,
		})
		go runOutboundDispatcher(leaderCtx, s.OutboundDispatcher, logger)
		go runDBStatsAnalyzer(leaderCtx, s.DBStatsService, metricsReg, logger)
		go runDBMonitor(leaderCtx, pools.App, metricsReg, logger)
		go runSoftDeletePurge(leaderCtx, s.LifecycleService, logger)
		go runAuditPruneScheduler(leaderCtx, s.Recorder, logger)
		go runUsageAlertEvaluator(leaderCtx, s.UsageAlertsService, logger)

		if cfg.HeartbeatWatcherEnabled {
			watcher := &systemcron.HeartbeatWatcher{
				Pool:    pools.App,
				Metrics: metricsReg,
				Timeout: time.Duration(cfg.HeartbeatWatcherTimeoutMinutes) * time.Minute,
				Tick:    time.Duration(cfg.HeartbeatWatcherTickSeconds) * time.Second,
				Logger:  logger,
			}
			go watcher.Start(leaderCtx)
		}

		go runFlowVersionArchiver(leaderCtx, pools.App, logger)

		if cfg.EdgeInferenceEnabled {
			inferencer := &systemcron.EdgeInferencer{
				Obs:          s.ObsService,
				Edges:        s.ObsEdgeService,
				Pool:         pools.App,
				Tick:         time.Duration(cfg.EdgeInferenceTickHours) * time.Hour,
				MaxPairs:     cfg.EdgeInferenceMaxPairs,
				ProjectBatch: cfg.EdgeInferenceProjectBatch,
				Logger:       logger,
			}
			go inferencer.Start(leaderCtx)
		}

		if cfg.FeedbackAggregatorEnabled {
			aggregator := &systemcron.FeedbackAggregator{
				Feedback: s.FeedbackService,
				Tick:     time.Duration(cfg.FeedbackAggregatorTickHours) * time.Hour,
				Days:     cfg.FeedbackAggregatorDays,
				Logger:   logger,
			}
			go aggregator.Start(leaderCtx)
		}

		if cfg.OrphanAuditEnabled {
			auditor := &systemcron.OrphanAuditor{
				Pool:    pools.App,
				Metrics: metricsReg,
				Tick:    24 * time.Hour,
				Batch:   1000,
				Logger:  logger,
			}
			go auditor.Start(leaderCtx)
		}

		scheduler.Run(leaderCtx)
	})

	return serverRunners{
		Scheduler:      scheduler,
		LeaderElection: leaderElection,
		SchedCancel:    schedCancel,
	}
}
