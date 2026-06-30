package systemcron

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthPoller escribe un heartbeat periódico en mcp_health_checks.
// Reemplaza el approach anterior de Django poll_mcp_health: el propio MCP
// server se auto-monitorea con un INSERT cada 60s. Bajo consumo: sin HTTP,
// sin metrics, sin tx — solo un INSERT directo a DB.
//
// Diseño: corre bajo leader election (RunAsLeader). No tiene flag propia de
// lock porque comparte el lock 1001 del cron scheduler. Default enabled.
type HealthPoller struct {
	Pool   *pgxpool.Pool
	Tick   time.Duration // default 60s
	Logger *slog.Logger
}

// Start corre el loop hasta que ctx se cancele.
func (p *HealthPoller) Start(ctx context.Context) {
	if p.Tick == 0 {
		p.Tick = 60 * time.Second
	}
	logger := p.Logger
	if logger == nil {
		logger = slog.Default()
	}

	logger.Info("health-poller started", slog.Duration("tick", p.Tick))

	ticker := time.NewTicker(p.Tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("health-poller stopping")
			return
		case <-ticker.C:
			p.runTick(ctx, logger)
		}
	}
}

func (p *HealthPoller) runTick(ctx context.Context, logger *slog.Logger) {
	if err := p.writeCheck(ctx, "up", 0, nil); err != nil {
		logger.Error("health-poller tick failed (writing down)", slog.Any("err", err))
		if writeErr := p.writeCheck(ctx, "down", 0, err); writeErr != nil {
			logger.Error("health-poller down-write also failed", slog.Any("err", writeErr))
		}
	}
}

// writeCheck hace INSERT directo en mcp_health_checks. Exportado para tests.
func (p *HealthPoller) writeCheck(ctx context.Context, status string, latencyMs int, checkErr error) error {
	query := `
		INSERT INTO mcp_health_checks (status, latency_ms, http_status, error)
		VALUES ($1, $2, $3, $4)
	`
	var errStr *string
	if checkErr != nil {
		s := checkErr.Error()
		errStr = &s
	}
	_, err := p.Pool.Exec(ctx, query, status, latencyMs, 200, errStr)
	return err
}
