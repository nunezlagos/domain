// orphan_runs_audit.go — issue-08.12.
//
// System cron diario que cuenta agent_runs orphan (sin flow_run_id ni
// metadata.standalone) e incrementa la métrica domain_agent_runs_orphan_total.
// Es la auditoría del enforcement híbrido del orquestador SDD (issue-08.10).
package systemcron

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/metrics"
)

// OrphanAuditor cuenta agent_runs con flow_run_id IS NULL y sin metadata.standalone
// (bypaseo del service-layer enforcement).
//
// REQ-42.3: el cursor last_ack_at ya NO se persiste en system_state (tabla
// dropeada). Se mantiene en memoria; tras un reinicio del proceso la primera
// pasada arranca desde 24h atrás (la auditoría es métrica best-effort, no
// requiere durabilidad del cursor).
type OrphanAuditor struct {
	Pool    *pgxpool.Pool
	Metrics *metrics.Registry
	Tick    time.Duration // default 24h
	Batch   int           // default 1000
	Logger  *slog.Logger

	mu        sync.Mutex
	lastAckAt time.Time
}

// orphanRow representa 1 conteo por org_id agregado en la query.
type orphanRow struct {
	Count int64
}

// Start corre el loop hasta ctx cancel. Asume llamado dentro de RunAsLeader.
func (a *OrphanAuditor) Start(ctx context.Context) {
	if a.Tick == 0 {
		a.Tick = 24 * time.Hour
	}
	if a.Batch == 0 {
		a.Batch = 1000
	}
	logger := a.Logger
	if logger == nil {
		logger = slog.Default()
	}

	logger.Info("orphan-runs-audit started", slog.Duration("tick", a.Tick))

	// Primera ejecución inmediata (útil al boot), luego periodic
	a.runTick(ctx, logger)

	ticker := time.NewTicker(a.Tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("orphan-runs-audit stopping")
			return
		case <-ticker.C:
			a.runTick(ctx, logger)
		}
	}
}

func (a *OrphanAuditor) runTick(ctx context.Context, logger *slog.Logger) {
	rows, lastSeenAt, err := a.Audit(ctx)
	if err != nil {
		logger.Error("orphan-runs-audit tick failed", slog.Any("err", err))
		a.observeTick("error")
		return
	}
	for _, r := range rows {
		logger.Warn("agent_runs orphan detected",
			slog.Int64("count", r.Count))
		a.observeOrphan(r)
	}
	if !lastSeenAt.IsZero() {
		a.setLastAck(lastSeenAt)
	}
	a.observeTick("ok")
}

// Audit ejecuta una pasada. Devuelve agregado por org_id + último created_at procesado.
// Exportado para tests + ejecución manual.
func (a *OrphanAuditor) Audit(ctx context.Context) ([]orphanRow, time.Time, error) {
	since := a.getLastAck()

	query := `
		SELECT
			COUNT(*),
			COALESCE(MAX(created_at), NOW()) AS last_seen
		FROM agent_runs
		WHERE flow_run_id IS NULL
		  AND (metadata->>'standalone' IS NULL OR metadata->>'standalone' != 'true')
		  AND created_at > $1
		  AND created_at <= NOW()
	`
	rows, err := a.Pool.Query(ctx, query, since)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("audit query: %w", err)
	}
	defer rows.Close()

	var out []orphanRow
	var maxSeen time.Time
	for rows.Next() {
		var r orphanRow
		var lastSeen time.Time
		if err := rows.Scan(&r.Count, &lastSeen); err != nil {
			return nil, time.Time{}, fmt.Errorf("scan: %w", err)
		}
		out = append(out, r)
		if lastSeen.After(maxSeen) {
			maxSeen = lastSeen
		}
	}
	if err := rows.Err(); err != nil {
		return nil, time.Time{}, fmt.Errorf("iterate: %w", err)
	}
	// Si no hubo rows, igual avanzamos last_ack a NOW() para no re-escanear ventana
	if maxSeen.IsZero() {
		maxSeen = time.Now()
	}
	return out, maxSeen, nil
}

// getLastAck devuelve el cursor en memoria. Primera pasada (cursor cero) →
// arranca desde 24h atrás. REQ-42.3: ya no se lee de system_state.
func (a *OrphanAuditor) getLastAck() time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.lastAckAt.IsZero() {
		return time.Now().Add(-24 * time.Hour)
	}
	return a.lastAckAt
}

// setLastAck avanza el cursor en memoria. REQ-42.3: ya no persiste en system_state.
func (a *OrphanAuditor) setLastAck(ts time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastAckAt = ts
}

func (a *OrphanAuditor) observeTick(result string) {
	if a.Metrics != nil && a.Metrics.OrphanAuditTicksTotal != nil {
		a.Metrics.OrphanAuditTicksTotal.WithLabelValues(result).Inc()
	}
}

func (a *OrphanAuditor) observeOrphan(r orphanRow) {
	if a.Metrics != nil && a.Metrics.AgentRunsOrphanTotal != nil {
		a.Metrics.AgentRunsOrphanTotal.
			WithLabelValues("", "bypass_service_layer").Add(float64(r.Count))
	}
}
