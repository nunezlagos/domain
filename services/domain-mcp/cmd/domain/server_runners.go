package main

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"time"

	"nunezlagos/domain/internal/config"
	"nunezlagos/domain/internal/metrics"
	flowrunner "nunezlagos/domain/internal/runner/flow"
	cronsched "nunezlagos/domain/internal/scheduler/cron"
	systemcron "nunezlagos/domain/internal/scheduler/cron/system"
	"nunezlagos/domain/internal/scheduler/leader"
	"nunezlagos/domain/internal/service/orchestrator"
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
		// REQ-54 issue-54.3 fix#1: worker de flows async del orquestador SDD.
		// Bajo leader (defensa en profundidad) + claim atómico SKIP LOCKED en
		// el repo (la garantía real contra doble ejecución).
		go runAsyncFlowWorker(leaderCtx, s.OrchestratorSvc, logger)

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

		if cfg.SkillMetricsEnabled {
			smAgg := &systemcron.SkillMetricsAggregator{
				Aggregator: s.SkillMetricsAggregator,
				Tick:       time.Duration(cfg.SkillMetricsTickHours) * time.Hour,
				Logger:     logger,
			}
			go smAgg.Start(leaderCtx)

			smRollup := &systemcron.SkillMetricsRollup{
				Aggregator:      s.SkillMetricsAggregator,
				Tick:            time.Duration(cfg.SkillMetricsRollupTickHours) * time.Hour,
				DailyRetention:  cfg.SkillMetricsDailyRetention,
				WeeklyRetention: cfg.SkillMetricsWeeklyRetention,
				Logger:          logger,
			}
			go smRollup.Start(leaderCtx)
		}

		if cfg.SkillJudgeEnabled {
			judge := &systemcron.SkillJudge{
				Aggregator: s.SkillJudgeAggregator,
				Weekday:    time.Weekday(cfg.SkillJudgeWeekday),
				Hour:       cfg.SkillJudgeHour,
				Logger:     logger,
			}
			go judge.Start(leaderCtx)
		}

		if cfg.ABTestEnabled {
			abAnalyzer := &systemcron.ABTestAnalyzer{
				Service:   s.SkillABTestService,
				Tick:      time.Duration(cfg.ABTestTickHours) * time.Hour,
				Alpha:     cfg.ABTestAlpha,
				AutoApply: cfg.ABTestAutoApply,
				Logger:    logger,
			}
			go abAnalyzer.Start(leaderCtx)
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

		if cfg.HealthPollerEnabled {
			poller := &systemcron.HealthPoller{
				Pool:   pools.App,
				Tick:   60 * time.Second,
				Logger: logger,
			}
			go poller.Start(leaderCtx)
		}

		if cfg.AuthAnomalyAuditEnabled {
			socAuditor := &systemcron.AuthAnomalyAuditor{
				Pool:      pools.App,
				Tick:      15 * time.Minute,
				Window:    15 * time.Minute,
				Threshold: 5,
				Logger:    logger,
			}
			go socAuditor.Start(leaderCtx)
		}

		scheduler.Run(leaderCtx)
	})

	return serverRunners{
		Scheduler:      scheduler,
		LeaderElection: leaderElection,
		SchedCancel:    schedCancel,
	}
}

// runAsyncFlowWorker lanza el worker de flows async del orquestador (REQ-54
// issue-54.3 fix#1). Config por env:
//   - DOMAIN_ASYNC_WORKER=off               → deshabilita el worker
//   - DOMAIN_ASYNC_WORKER_CONCURRENCY=N     → flows simultáneos (default 1)
//
// El worker se auto-deshabilita (con log) si el orquestador no tiene LLM o si
// el Repo no soporta claim atómico — el server arranca normal igual.
func runAsyncFlowWorker(ctx context.Context, orch *orchestrator.Service, logger *slog.Logger) {
	if orch == nil {
		return
	}
	if os.Getenv("DOMAIN_ASYNC_WORKER") == "off" {
		logger.Info("async flow worker: deshabilitado por DOMAIN_ASYNC_WORKER=off")
		return
	}
	conc := 1
	if v := os.Getenv("DOMAIN_ASYNC_WORKER_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			conc = n
		}
	}
	orch.RunAsyncWorker(ctx, orchestrator.AsyncWorkerConfig{
		Concurrency: conc,
		Logger:      logger,
	})
}
