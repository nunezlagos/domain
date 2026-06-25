package main

import (
	"context"
	"log/slog"
	"time"

	"nunezlagos/domain/internal/config"
	"nunezlagos/domain/internal/db"
	"nunezlagos/domain/internal/metrics"
)

// serverPools agrupa los tres pgxpool abiertos por buildPools.
type serverPools struct {
	*db.Pools
}

// buildPools abre los tres pools de BD (App, Auth, ReadOnly) y registra el
// lag monitor si hay réplica de lectura configurada. Devuelve un cleanup que
// cierra los pools — el caller debe diferirlo.
func buildPools(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger,
	metricsReg *metrics.Registry,
) (serverPools, func(), error) {
	pools, err := db.OpenProductionWithReplica(ctx, cfg.DatabaseURL, cfg.DatabaseAuthURL, cfg.DatabaseReadOnlyURL)
	if err != nil {
		return serverPools{}, nil, err
	}

	metrics.RunPoolStatsReporter(ctx, metricsReg, pools.App, pools.Auth, pools.ReadOnly, logger)

	if cfg.DatabaseAuthURL == "" && cfg.Env != "dev" {
		logger.Warn("DOMAIN_DATABASE_AUTH_URL not set — auth pool reuses runtime user (NOT recommended outside dev)")
	}

	if pools.ReadOnly != nil {
		pools.LagMonitor = &db.LagMonitor{
			Pool: pools.ReadOnly, PollInterval: 30 * time.Second,
			ThresholdSecs: 10.0, Logger: logger,
			MetricsCB: func(lag float64) {
				metricsReg.ReplicationLagSeconds.Set(lag)
				if lag > 10.0 {
					metricsReg.ReplicaFallbackTotal.Inc()
				}
			},
		}
		go pools.LagMonitor.Run(ctx)
		logger.Info("read replica configured with lag monitor",
			slog.Float64("threshold_secs", 10.0))
	} else {
		logger.Info("no read replica configured — all reads go to primary")
	}

	cleanup := func() { pools.Close() }
	return serverPools{pools}, cleanup, nil
}
