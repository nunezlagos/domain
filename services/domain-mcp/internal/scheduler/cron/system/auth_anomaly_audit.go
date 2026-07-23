// auth_anomaly_audit.go — DOMAINSERV-82 H3.
//
// System cron SOC: cada Tick detecta brute-force sobre auth_events (fallos
// agrupados por (email_attempted, ip) que superan Threshold dentro de Window)
// y emite slog.Warn con email_hash+ip+count — sin secretos (policy
// secrets-redaction). Los apikey_auth_failed (email NULL) agrupan por ip.
// Sink = log estructurado (base para Loki, DOMAINSERV-81). Menos carga que un
// listener real-time: una query indexada (auth_events_email_ip_idx) por tick.
package systemcron

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthAnomalyAuditor struct {
	Pool      *pgxpool.Pool
	Tick      time.Duration // default 15m
	Window    time.Duration // default 15m
	Threshold int           // default 5
	Logger    *slog.Logger
}

// authAnomaly = un cluster (email_attempted, ip) que superó el umbral de fallos.
type authAnomaly struct {
	EmailHash string // "" si apikey (email NULL)
	IP        string
	Count     int64
}

func (a *AuthAnomalyAuditor) defaults() (time.Duration, int) {
	window := a.Window
	if window == 0 {
		window = 15 * time.Minute
	}
	threshold := a.Threshold
	if threshold == 0 {
		threshold = 5
	}
	return window, threshold
}

func (a *AuthAnomalyAuditor) Start(ctx context.Context) {
	if a.Tick == 0 {
		a.Tick = 15 * time.Minute
	}
	logger := a.Logger
	if logger == nil {
		logger = slog.Default()
	}
	_, threshold := a.defaults()
	logger.Info("auth-anomaly-audit started", slog.Duration("tick", a.Tick), slog.Int("threshold", threshold))

	a.runTick(ctx, logger)
	ticker := time.NewTicker(a.Tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("auth-anomaly-audit stopping")
			return
		case <-ticker.C:
			a.runTick(ctx, logger)
		}
	}
}

func (a *AuthAnomalyAuditor) runTick(ctx context.Context, logger *slog.Logger) {
	window, _ := a.defaults()
	anomalies, err := a.Audit(ctx)
	if err != nil {
		logger.Error("auth-anomaly-audit tick failed", slog.Any("err", err))
		return
	}
	for _, an := range anomalies {
		logger.Warn("auth brute-force sospechoso",
			slog.String("email_hash", an.EmailHash),
			slog.String("ip", an.IP),
			slog.Int64("failed_count", an.Count),
			slog.Duration("window", window))
	}
}

// Audit ejecuta una pasada: fallos de auth en la ventana agrupados por
// (email_attempted, ip) que superan Threshold. Exportado para tests. Usa el
// índice parcial auth_events_email_ip_idx (WHERE success = FALSE).
func (a *AuthAnomalyAuditor) Audit(ctx context.Context) ([]authAnomaly, error) {
	window, threshold := a.defaults()
	rows, err := a.Pool.Query(ctx, `
		SELECT COALESCE(email_attempted, ''), COALESCE(host(ip), ''), COUNT(*)
		FROM auth_events
		WHERE success = FALSE
		  AND created_at > NOW() - ($1 * interval '1 second')
		GROUP BY email_attempted, ip
		HAVING COUNT(*) >= $2
		ORDER BY COUNT(*) DESC`,
		int(window.Seconds()), threshold)
	if err != nil {
		return nil, fmt.Errorf("audit query: %w", err)
	}
	defer rows.Close()

	var out []authAnomaly
	for rows.Next() {
		var an authAnomaly
		var email string
		if err := rows.Scan(&email, &an.IP, &an.Count); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		an.EmailHash = hashEmail(email)
		out = append(out, an)
	}
	return out, rows.Err()
}

// hashEmail devuelve sha256[:8] hex del email, o "" si vacío (apikey failure).
func hashEmail(email string) string {
	if email == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(email))
	return hex.EncodeToString(sum[:])[:8]
}
