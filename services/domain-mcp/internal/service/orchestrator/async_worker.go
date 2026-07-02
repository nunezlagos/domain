package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
)

// REQ-54 issue-54.3 fix#1: worker de flows async.
//
// ProcessAsyncFlowRun existía completo pero sin caller en producción: los flows
// creados en modo async quedaban 'pending' para siempre. Este worker es el
// dispatcher, en versión MVP conservadora:
//
//   - Claim ATÓMICO por flow (FOR UPDATE SKIP LOCKED, pending→running en la
//     misma sentencia): dos workers no pueden tomar el mismo flow. Es la
//     defensa principal contra doble ejecución / doble gasto de LLM.
//   - Corre bajo leader election (ver cmd/domain/server_runners.go): solo el
//     líder lo ejecuta — defensa en profundidad, no la garantía.
//   - Solo procesa flows creados EXPLÍCITAMENTE en modo async (el claim filtra
//     por cursor->>'mode'='async'); jamás toca flows interactivos.
//   - Concurrencia acotada (default 1) para que un backlog de flows async no
//     dispare gasto de tokens descontrolado al arrancar.
//   - Requiere LLM y un Repo con soporte de claim; si falta cualquiera, el
//     worker se deshabilita con un log y el server arranca normal.

// AsyncFlowClaimer es la capacidad OPCIONAL del Repository de tomar el próximo
// flow async pendiente de forma atómica. Es una interfaz separada (no parte de
// Repository) para no obligar a los fakes de test a implementarla.
type AsyncFlowClaimer interface {
	ClaimNextPendingAsyncFlow(ctx context.Context, workerID string) (uuid.UUID, bool, error)
}

// AsyncWorkerConfig parametriza el worker. Zero-value = defaults conservadores.
type AsyncWorkerConfig struct {
	Interval    time.Duration // período de polling; default 15s
	Concurrency int           // flows simultáneos; default 1
	WorkerID    string        // identificador para flow_runs.worker_id; default host+pid
	Logger      *slog.Logger
}

// RunAsyncWorker corre el loop de polling hasta que ctx se cancele. Está
// pensado para lanzarse como goroutine en el boot, bajo el lock de leader.
func (s *Service) RunAsyncWorker(ctx context.Context, cfg AsyncWorkerConfig) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if s.LLM == nil {
		logger.Info("async worker: deshabilitado (LLM factory no configurado)")
		return
	}
	claimer, ok := s.Repo.(AsyncFlowClaimer)
	if !ok {
		logger.Info("async worker: deshabilitado (Repo sin soporte de claim atómico)")
		return
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 15 * time.Second
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	if cfg.WorkerID == "" {
		host, _ := os.Hostname()
		cfg.WorkerID = fmt.Sprintf("async-worker-%s-%d", host, os.Getpid())
	}
	logger.Info("async worker: iniciado",
		"interval", cfg.Interval.String(),
		"concurrency", cfg.Concurrency,
		"worker_id", cfg.WorkerID)

	sem := make(chan struct{}, cfg.Concurrency)
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.drainAsyncFlows(ctx, claimer, sem, cfg.WorkerID, logger)
		}
	}
}

// drainAsyncFlows toma flows pendientes mientras haya slots de concurrencia
// libres y los procesa en goroutines. Corta cuando no hay más pendientes o no
// hay slots (los que queden esperan al próximo tick).
func (s *Service) drainAsyncFlows(ctx context.Context, claimer AsyncFlowClaimer,
	sem chan struct{}, workerID string, logger *slog.Logger,
) {
	for {
		select {
		case sem <- struct{}{}: // slot adquirido
		default:
			return // sin slots: esperar próximo tick
		}
		id, ok, err := claimer.ClaimNextPendingAsyncFlow(ctx, workerID)
		if err != nil {
			<-sem
			logger.Error("async worker: claim falló", "error", err)
			return
		}
		if !ok {
			<-sem
			return // no hay flows async pendientes
		}
		go func(fid uuid.UUID) {
			defer func() { <-sem }()
			if err := s.ProcessAsyncFlowRun(ctx, fid); err != nil {
				logger.Error("async worker: flow falló", "flow_run_id", fid.String(), "error", err)
				// El flow quedó claimed en 'running'; si ProcessAsyncFlowRun no
				// llegó a propagar el fallo (error temprano: template/provider),
				// lo marcamos failed para que NUNCA quede 'running' colgado.
				// Idempotente si propagateFlowStatusAfterFailure ya lo marcó.
				_ = s.Repo.SetFlowRunError(ctx, fid, err.Error())
				_ = s.Repo.UpdateFlowRunStatus(ctx, fid, "failed")
			}
		}(id)
	}
}
