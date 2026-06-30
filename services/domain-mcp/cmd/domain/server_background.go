package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"nunezlagos/domain/internal/cache"
	"nunezlagos/domain/internal/config"
	"nunezlagos/domain/internal/httpserver"
	"nunezlagos/domain/internal/metrics"
)

// startBackground inicia el servidor HTTP, registra el manejador de señales y
// arranca las goroutines de reporte que no requieren el lock de leader.
// Bloquea hasta que el servidor termina (cierre limpio o error fatal).
func startBackground(
	ctx context.Context,
	cfg *config.Config,
	pools serverPools,
	s *serverServices,
	runners serverRunners,
	metricsReg *metrics.Registry,
	handler http.Handler,
	queryCacheLRU *cache.LRU,
	logger *slog.Logger,
) {
	addr := cfg.HTTPBind + ":" + strconv.Itoa(cfg.HTTPPort)

	go runReq70Reporter(ctx, metricsReg, queryCacheLRU, s.EventBus, pools.App, logger)

	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  time.Duration(cfg.HTTPReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.HTTPWriteTimeoutSeconds) * time.Second,
	}

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-shutdownCh
		shutdownStart := time.Now()
		logger.Info("shutdown signal received", slog.String("signal", sig.String()))

		grace := 5 * time.Second
		if v := os.Getenv("DOMAIN_SHUTDOWN_GRACE_SECONDS"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 25 {
				grace = time.Duration(n) * time.Second
			}
		}
		httpserver.ShuttingDown.Store(true)
		logger.Info("readiness flipped → unhealthy; waiting ELB drain",
			slog.Duration("grace", grace))
		time.Sleep(grace)

		httpCtx, httpCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer httpCancel()
		forced := false
		if err := srv.Shutdown(httpCtx); err != nil {
			logger.Warn("http shutdown forced after timeout", slog.Any("err", err))
			forced = true
		}

		runners.SchedCancel()

		duration := time.Since(shutdownStart).Seconds()
		logger.Info("graceful shutdown complete",
			slog.Float64("duration_s", duration),
			slog.Bool("forced", forced))
	}()

	listenErrCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			listenErrCh <- err
		} else {
			listenErrCh <- nil
		}
	}()

	go func() {
		time.Sleep(2 * time.Second)
		for i := 0; i < 3; i++ {
			if err := httpserver.ProbeHealth(cfg.HTTPPort); err == nil {
				return
			}
			time.Sleep(1 * time.Second)
		}
		logger.Error("FATAL: health-check post-bind failed 3x — listener not responding",
			slog.Int("port", cfg.HTTPPort))
		os.Exit(1)
	}()

	if err := <-listenErrCh; err != nil {
		logger.Error("FATAL: HTTP listener failed", slog.Any("err", err))
		os.Exit(1)
	}
}
