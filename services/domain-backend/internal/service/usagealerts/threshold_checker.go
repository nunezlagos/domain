package usagealerts

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CostAlert struct {
	TotalUSD     float64
	ThresholdUSD float64
	Breakdown    []CostBreakdownItem
	AlertDate    time.Time
}

type CostBreakdownItem struct {
	Provider string
	Model    string
	CostUSD  float64
}

func CheckThresholds(ctx context.Context, pool *pgxpool.Pool) ([]CostAlert, error) {
	rows, err := pool.Query(ctx, `
		SELECT
			SUM(ocl.cost_usd) AS total,
			COALESCE((SELECT daily_usd FROM org_cost_alert_thresholds LIMIT 1), 100.00) AS threshold
		FROM cost_logs ocl
		WHERE ocl.created_at >= date_trunc('day', NOW() AT TIME ZONE 'UTC')
		  AND ocl.created_at < date_trunc('day', NOW() AT TIME ZONE 'UTC') + INTERVAL '1 day'
		HAVING SUM(ocl.cost_usd) >= COALESCE((SELECT daily_usd FROM org_cost_alert_thresholds LIMIT 1), 100.00)
	`)
	if err != nil {
		return nil, fmt.Errorf("check thresholds query: %w", err)
	}
	defer rows.Close()

	var alerts []CostAlert
	for rows.Next() {
		var a CostAlert
		if err := rows.Scan(&a.TotalUSD, &a.ThresholdUSD); err != nil {
			return nil, fmt.Errorf("scan alert: %w", err)
		}
		a.AlertDate = time.Now().UTC().Truncate(24 * time.Hour)
		alerts = append(alerts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range alerts {
		breakdown, err := fetchBreakdown(ctx, pool, uuid.Nil, alerts[i].AlertDate)
		if err != nil {
			return nil, fmt.Errorf("breakdown: %w", err)
		}
		alerts[i].Breakdown = breakdown
	}
	return alerts, nil
}

func fetchBreakdown(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, day time.Time) ([]CostBreakdownItem, error) {
	rows, err := pool.Query(ctx, `
		SELECT provider, model, SUM(cost_usd) AS sub
		FROM cost_logs
		WHERE created_at >= $1
		  AND created_at < $1 + INTERVAL '1 day'
		GROUP BY provider, model
		ORDER BY provider, model
	`, day)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []CostBreakdownItem
	for rows.Next() {
		var item CostBreakdownItem
		if err := rows.Scan(&item.Provider, &item.Model, &item.CostUSD); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func SendAlerts(ctx context.Context, pool *pgxpool.Pool, alerts []CostAlert, sender EmailSender, logger *slog.Logger) (int, error) {
	sent := 0
	for _, a := range alerts {
		tag, err := pool.Exec(ctx, `
			INSERT INTO cost_alerts_sent (alert_date, amount_usd)
			VALUES ($1, $2)
			ON CONFLICT (alert_date) DO NOTHING
		`, a.AlertDate, a.TotalUSD)
		if err != nil {
			return sent, fmt.Errorf("insert cost_alerts_sent: %w", err)
		}
		if tag.RowsAffected() == 0 {
			continue
		}
		sent++

		if sender == nil {
			if logger != nil {
				logger.Warn("cost alert would be sent but no email sender configured",
					"total_usd", a.TotalUSD,
					"threshold", a.ThresholdUSD,
				)
			}
			continue
		}

		subject, body := RenderEmail(a)
		if logger != nil {
			logger.Info("sending cost alert email",
				"subject", subject,
			)
		}
		if err := sender.SendAlertEmail(ctx, nil, subject, body); err != nil {
			if logger != nil {
				logger.Warn("failed to send cost alert email",
					"error", err.Error(),
				)
			}
		}
	}
	return sent, nil
}

func RenderEmail(a CostAlert) (subject, body string) {
	date := a.AlertDate.Format("2006-01-02")
	subject = fmt.Sprintf("[domain] cost alert: exceeded $%.2f/day (now $%.2f)",
		a.ThresholdUSD, a.TotalUSD)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Date (UTC): %s\n", date))
	b.WriteString(fmt.Sprintf("Threshold: $%.2f\n", a.ThresholdUSD))
	b.WriteString(fmt.Sprintf("Current spend: $%.2f\n", a.TotalUSD))

	if len(a.Breakdown) > 0 {
		b.WriteString("\nBreakdown by provider:\n")
		byProvider := map[string]float64{}
		for _, item := range a.Breakdown {
			byProvider[item.Provider] += item.CostUSD
		}
		for prov, total := range byProvider {
			var modelLines []string
			for _, item := range a.Breakdown {
				if item.Provider == prov {
					modelLines = append(modelLines, fmt.Sprintf("  %s: $%.2f", item.Model, item.CostUSD))
				}
			}
			b.WriteString(fmt.Sprintf("  %s: $%.2f\n", prov, total))
			for _, line := range modelLines {
				b.WriteString("    " + line + "\n")
			}
		}
	}
	body = b.String()
	return subject, body
}

// EnableCostThreshold setea el threshold de costo diario global (single-org,
// sin organization_id). El param orgID se conserva por compat de signatura.
func EnableCostThreshold(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, dailyUSD float64) error {
	_ = orgID
	_, err := pool.Exec(ctx, `
		UPDATE org_cost_alert_thresholds SET daily_usd = $1, updated_at = NOW()
	`, dailyUSD)
	if err != nil {
		return err
	}
	// Sin row aún: insertar uno (single-org global).
	_, err = pool.Exec(ctx, `
		INSERT INTO org_cost_alert_thresholds (daily_usd)
		SELECT $1
		WHERE NOT EXISTS (SELECT 1 FROM org_cost_alert_thresholds)
	`, dailyUSD)
	return err
}

// GetCostThreshold lee el threshold de costo diario global (single-org).
// El param orgID se conserva por compat de signatura.
func GetCostThreshold(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID) (float64, error) {
	_ = orgID
	var dailyUSD float64
	err := pool.QueryRow(ctx,
		`SELECT COALESCE(daily_usd, 100.00) FROM org_cost_alert_thresholds LIMIT 1`,
	).Scan(&dailyUSD)
	if err != nil {
		return 100.00, nil
	}
	return dailyUSD, nil
}
