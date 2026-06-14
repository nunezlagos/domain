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
	OrganizationID uuid.UUID
	TotalUSD       float64
	ThresholdUSD   float64
	Breakdown      []CostBreakdownItem
	AlertDate      time.Time
}

type CostBreakdownItem struct {
	Provider string
	Model    string
	CostUSD  float64
}

func CheckThresholds(ctx context.Context, pool *pgxpool.Pool) ([]CostAlert, error) {
	rows, err := pool.Query(ctx, `
		SELECT
			ocl.organization_id,
			SUM(ocl.cost_usd) AS total,
			COALESCE(t.daily_usd, 100.00) AS threshold
		FROM cost_logs ocl
		LEFT JOIN org_cost_alert_thresholds t ON t.organization_id = ocl.organization_id
		WHERE ocl.created_at >= date_trunc('day', NOW() AT TIME ZONE 'UTC')
		  AND ocl.created_at < date_trunc('day', NOW() AT TIME ZONE 'UTC') + INTERVAL '1 day'
		GROUP BY ocl.organization_id, t.daily_usd
		HAVING SUM(ocl.cost_usd) >= COALESCE(t.daily_usd, 100.00)
	`)
	if err != nil {
		return nil, fmt.Errorf("check thresholds query: %w", err)
	}
	defer rows.Close()

	var alerts []CostAlert
	for rows.Next() {
		var a CostAlert
		if err := rows.Scan(&a.OrganizationID, &a.TotalUSD, &a.ThresholdUSD); err != nil {
			return nil, fmt.Errorf("scan alert: %w", err)
		}
		a.AlertDate = time.Now().UTC().Truncate(24 * time.Hour)
		alerts = append(alerts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range alerts {
		breakdown, err := fetchBreakdown(ctx, pool, alerts[i].OrganizationID, alerts[i].AlertDate)
		if err != nil {
			return nil, fmt.Errorf("breakdown for %s: %w", alerts[i].OrganizationID, err)
		}
		alerts[i].Breakdown = breakdown
	}
	return alerts, nil
}

func fetchBreakdown(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, day time.Time) ([]CostBreakdownItem, error) {
	rows, err := pool.Query(ctx, `
		SELECT provider, model, SUM(cost_usd) AS sub
		FROM cost_logs
		WHERE organization_id = $1
		  AND created_at >= $2
		  AND created_at < $2 + INTERVAL '1 day'
		GROUP BY provider, model
		ORDER BY provider, model
	`, orgID, day)
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
			INSERT INTO cost_alerts_sent (organization_id, alert_date, amount_usd)
			VALUES ($1, $2, $3)
			ON CONFLICT (organization_id, alert_date) DO NOTHING
		`, a.OrganizationID, a.AlertDate, a.TotalUSD)
		if err != nil {
			return sent, fmt.Errorf("insert cost_alerts_sent for %s: %w", a.OrganizationID, err)
		}
		if tag.RowsAffected() == 0 {
			continue
		}
		sent++

		if sender == nil {
			if logger != nil {
				logger.Warn("cost alert for org would be sent but no email sender configured",
					"org_id", a.OrganizationID,
					"total_usd", a.TotalUSD,
					"threshold", a.ThresholdUSD,
				)
			}
			continue
		}

		subject, body := RenderEmail(a)
		if logger != nil {
			logger.Info("sending cost alert email",
				"org_id", a.OrganizationID,
				"subject", subject,
			)
		}
		if err := sender.SendAlertEmail(ctx, nil, subject, body); err != nil {
			if logger != nil {
				logger.Warn("failed to send cost alert email",
					"org_id", a.OrganizationID,
					"error", err.Error(),
				)
			}
		}
	}
	return sent, nil
}

func RenderEmail(a CostAlert) (subject, body string) {
	date := a.AlertDate.Format("2006-01-02")
	subject = fmt.Sprintf("[domain] cost alert: org %s exceeded $%.2f/day (now $%.2f)",
		a.OrganizationID, a.ThresholdUSD, a.TotalUSD)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Organization: %s\n", a.OrganizationID))
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

func EnableCostThreshold(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, dailyUSD float64) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO org_cost_alert_thresholds (organization_id, daily_usd)
		VALUES ($1, $2)
		ON CONFLICT (organization_id) DO UPDATE SET daily_usd = $2, updated_at = NOW()
	`, orgID, dailyUSD)
	return err
}

func GetCostThreshold(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID) (float64, error) {
	var dailyUSD float64
	err := pool.QueryRow(ctx,
		`SELECT COALESCE(daily_usd, 100.00) FROM org_cost_alert_thresholds WHERE organization_id = $1`,
		orgID).Scan(&dailyUSD)
	if err != nil {
		return 100.00, nil
	}
	return dailyUSD, nil
}
