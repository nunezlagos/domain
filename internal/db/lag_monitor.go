package db

import (
	"context"
	"log/slog"
	"math"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LagMonitor consulta periódicamente la replica para conocer el replication lag.
//
// HU-25.9 escenario 3: cron 30s + métrica + alerta si >5s sostenido.
// HU-25.9 escenario 4: fallback automático a primary si lag >ThresholdSecs.
type LagMonitor struct {
	Pool          *pgxpool.Pool
	PollInterval  time.Duration // default 30s
	ThresholdSecs float64       // default 10.0 (sobre esto → fallback a primary)
	Logger        *slog.Logger

	// MetricsCB se invoca en cada tick con el lag medido (HU-25.9 métricas).
	MetricsCB func(lag float64)

	// lagSeconds es el último valor medido. atomic para acceso lock-free.
	lagSeconds atomic.Uint64 // float64 bits
}

// Run bloquea hasta ctx.Done() consultando lag periódicamente.
func (m *LagMonitor) Run(ctx context.Context) {
	if m.Pool == nil {
		return
	}
	if m.PollInterval == 0 {
		m.PollInterval = 30 * time.Second
	}
	if m.ThresholdSecs == 0 {
		m.ThresholdSecs = 10.0
	}
	ticker := time.NewTicker(m.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.tick(ctx)
		}
	}
}

func (m *LagMonitor) tick(ctx context.Context) {
	var lag float64
	err := m.Pool.QueryRow(ctx,
		`SELECT COALESCE(EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp())), 0)`,
	).Scan(&lag)
	if err != nil {
		if m.Logger != nil {
			m.Logger.WarnContext(ctx, "replica lag query failed",
				slog.String("error", err.Error()))
		}
		return
	}
	m.setLag(lag)
	if m.MetricsCB != nil {
		m.MetricsCB(lag)
	}
	if m.Logger != nil && lag > m.ThresholdSecs {
		m.Logger.WarnContext(ctx, "replica lag exceeded threshold",
			slog.Float64("lag_seconds", lag),
			slog.Float64("threshold_secs", m.ThresholdSecs))
	}
}

// LagSeconds retorna el último valor medido.
func (m *LagMonitor) LagSeconds() float64 {
	return floatFromBits(m.lagSeconds.Load())
}

// IsDegraded retorna true si el lag excede el threshold — callers deben
// preferir App pool en ese caso.
func (m *LagMonitor) IsDegraded() bool {
	return m.LagSeconds() > m.ThresholdSecs
}

func (m *LagMonitor) setLag(v float64) {
	m.lagSeconds.Store(floatToBits(v))
}

func floatToBits(v float64) uint64    { return math.Float64bits(v) }
func floatFromBits(b uint64) float64  { return math.Float64frombits(b) }
