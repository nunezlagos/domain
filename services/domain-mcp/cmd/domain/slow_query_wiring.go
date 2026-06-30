package main

import (
	"log/slog"

	"nunezlagos/domain/internal/mcp/server"
	"nunezlagos/domain/internal/observability"
)

// setupSlowQueryTracer construye el SlowQueryTracer con su store, listo
// para ser encadenado al pgx.QueryTracer del pool via db.SetObservabilityTracer.
//
// Uso:
//
//	slowTracer, store := setupSlowQueryTracer(logger)
//	db.SetObservabilityTracer(slowTracer)        // antes de db.Open*
//	pools, ... := buildPools(...)                  // abre pools
//	store.SetPool(pools.App)                       // atacha el pool al store
//
// El SlowQueryTracer encapsula el inner tracer (SQLErrorCaptureTracer, HU 51.1)
// para mantener compatibilidad con la captura de errores SQL en tx aborted
// (cascade 25P02). thresholdMs<0 usa el default (100ms); DOMAIN_SQL_SLOW_THRESHOLD_MS
// override via env.
func setupSlowQueryTracer(logger *slog.Logger) (*observability.SlowQueryTracer, *observability.PGSlowQueryStore) {
	store := &observability.PGSlowQueryStore{}
	tracer := observability.NewFromEnv(
		mcpserver.SQLErrorCaptureTracer(),
		store,
		logger,
		0,
	)
	return tracer, store
}
