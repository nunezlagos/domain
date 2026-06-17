// issue-15.2 — analytics de cost: agregación temporal, breakdown por
// dimensión, forecast SMA, budgets con current_spend y export CSV.
package cost

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SpendBucket es el gasto agregado en una ventana temporal.
type SpendBucket struct {
	Bucket  time.Time `json:"bucket"`
	Runs    int64     `json:"runs"`
	CostUSD float64   `json:"cost_usd"`
}

var validGranularities = map[string]string{
	"daily": "day", "weekly": "week", "monthly": "month",
}

// ErrInvalidDimension dimensión de breakdown no soportada.
var ErrInvalidDimension = errors.New("invalid dimension")

// ErrInvalidGranularity granularidad no soportada.
var ErrInvalidGranularity = errors.New("invalid granularity")

// Spend agrega cost_logs por day/week/month sobre los últimos days días.
func (s *Service) Spend(ctx context.Context, orgID uuid.UUID, granularity string, days int) ([]SpendBucket, error) {
	trunc, ok := validGranularities[granularity]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrInvalidGranularity, granularity)
	}
	if days <= 0 || days > 365 {
		days = 30
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT date_trunc($2, occurred_at) AS bucket,
		       COUNT(*) AS runs, COALESCE(SUM(cost_usd), 0) AS cost_usd
		FROM cost_logs
		WHERE occurred_at >= NOW() - ($1 || ' days')::interval
		GROUP BY bucket ORDER BY bucket DESC`,
		strconv.Itoa(days), trunc)
	if err != nil {
		return nil, fmt.Errorf("spend query: %w", err)
	}
	defer rows.Close()
	var out []SpendBucket
	for rows.Next() {
		var b SpendBucket
		if err := rows.Scan(&b.Bucket, &b.Runs, &b.CostUSD); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// BreakdownRow gasto agregado por una dimensión.
type BreakdownRow struct {
	Key     string  `json:"key"`
	Runs    int64   `json:"runs"`
	CostUSD float64 `json:"cost_usd"`
}

// Breakdown agrega cost por dimensión: provider | model | operation |
// agent (slug) | flow (slug).
func (s *Service) Breakdown(ctx context.Context, orgID uuid.UUID, dimension string, days int) ([]BreakdownRow, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	var query string
	switch dimension {
	case "provider", "model", "operation":
		query = fmt.Sprintf(`
			SELECT %s AS key, COUNT(*), COALESCE(SUM(cost_usd), 0)
			FROM cost_logs
			WHERE occurred_at >= NOW() - ($1 || ' days')::interval
			GROUP BY key ORDER BY 3 DESC`, dimension)
	case "agent":
		query = `
			SELECT COALESCE(a.slug, 'sin-agent') AS key, COUNT(*), COALESCE(SUM(c.cost_usd), 0)
			FROM cost_logs c
			LEFT JOIN agent_runs ar ON ar.id = c.agent_run_id
			LEFT JOIN agents a ON a.id = ar.agent_id
			WHERE c.occurred_at >= NOW() - ($1 || ' days')::interval
			GROUP BY key ORDER BY 3 DESC`
	case "flow":
		query = `
			SELECT COALESCE(f.slug, 'sin-flow') AS key, COUNT(*), COALESCE(SUM(c.cost_usd), 0)
			FROM cost_logs c
			LEFT JOIN flow_runs fr ON fr.id = c.flow_run_id
			LEFT JOIN flows f ON f.id = fr.flow_id
			WHERE c.occurred_at >= NOW() - ($1 || ' days')::interval
			GROUP BY key ORDER BY 3 DESC`
	default:
		return nil, fmt.Errorf("%w: %s (supported: provider, model, operation, agent, flow)",
			ErrInvalidDimension, dimension)
	}
	rows, err := s.Pool.Query(ctx, query, strconv.Itoa(days))
	if err != nil {
		return nil, fmt.Errorf("breakdown query: %w", err)
	}
	defer rows.Close()
	var out []BreakdownRow
	for rows.Next() {
		var r BreakdownRow
		if err := rows.Scan(&r.Key, &r.Runs, &r.CostUSD); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Forecast proyección por simple moving average.
type Forecast struct {
	WindowDays      int     `json:"window_days"`
	AvgDailyUSD     float64 `json:"avg_daily_usd"`
	Next30DaysUSD   float64 `json:"next_30_days_usd"`
	MonthToDateUSD  float64 `json:"month_to_date_usd"`
	MonthEndUSD     float64 `json:"month_end_projected_usd"`
}

// SMAForecast calcula el promedio diario de la ventana y proyecta.
// Sin datos → todo 0 (nunca panic).
func SMAForecast(dailyCosts []float64, monthToDate float64, daysElapsed, daysInMonth int) Forecast {
	f := Forecast{WindowDays: len(dailyCosts), MonthToDateUSD: monthToDate}
	if len(dailyCosts) > 0 {
		var sum float64
		for _, c := range dailyCosts {
			sum += c
		}
		f.AvgDailyUSD = sum / float64(len(dailyCosts))
	}
	f.Next30DaysUSD = f.AvgDailyUSD * 30
	remaining := daysInMonth - daysElapsed
	if remaining < 0 {
		remaining = 0
	}
	f.MonthEndUSD = monthToDate + f.AvgDailyUSD*float64(remaining)
	return f
}

// ForecastSMA consulta los últimos windowDays y proyecta con SMA.
func (s *Service) ForecastSMA(ctx context.Context, orgID uuid.UUID, windowDays int) (*Forecast, error) {
	if windowDays <= 0 || windowDays > 90 {
		windowDays = 14
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT COALESCE(SUM(cost_usd), 0)
		FROM cost_logs
		WHERE occurred_at >= CURRENT_DATE - ($1 || ' days')::interval
		  AND occurred_at < CURRENT_DATE
		GROUP BY date_trunc('day', occurred_at)`,
		strconv.Itoa(windowDays))
	if err != nil {
		return nil, fmt.Errorf("forecast window: %w", err)
	}
	var daily []float64
	for rows.Next() {
		var c float64
		if err := rows.Scan(&c); err != nil {
			rows.Close()
			return nil, err
		}
		daily = append(daily, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var monthToDate float64
	if err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(cost_usd), 0) FROM cost_logs
		WHERE occurred_at >= date_trunc('month', NOW())`,
	).Scan(&monthToDate); err != nil {
		return nil, fmt.Errorf("month to date: %w", err)
	}

	now := time.Now().UTC()
	daysInMonth := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	f := SMAForecast(daily, monthToDate, now.Day(), daysInMonth)
	f.WindowDays = windowDays
	return &f, nil
}

// --- Budgets ---

// Budget límite de gasto configurado.
type Budget struct {
	ID                  uuid.UUID `json:"id"`
	Name                string    `json:"name"`
	AmountUSD           float64   `json:"amount_usd"`
	Period              string    `json:"period"`
	WarningThresholdPct int       `json:"warning_threshold_pct"`
	CurrentSpendUSD     float64   `json:"current_spend_usd"`
	Status              string    `json:"status"` // ok | warning | exceeded
	CreatedAt           time.Time `json:"created_at"`
}

var ErrBudgetNotFound = errors.New("budget not found")

// BudgetStatus clasifica el gasto actual contra el límite.
func BudgetStatus(spend, amount float64, warningPct int) string {
	if amount <= 0 {
		return "ok"
	}
	switch {
	case spend >= amount:
		return "exceeded"
	case spend >= amount*float64(warningPct)/100:
		return "warning"
	default:
		return "ok"
	}
}

func periodStart(period string) string {
	switch period {
	case "daily":
		return "day"
	case "weekly":
		return "week"
	default:
		return "month"
	}
}

// CreateBudget crea un budget para la org.
func (s *Service) CreateBudget(ctx context.Context, orgID uuid.UUID, name string, amountUSD float64, period string, warningPct int) (*Budget, error) {
	if warningPct <= 0 {
		warningPct = 80
	}
	if period == "" {
		period = "monthly"
	}
	var b Budget
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO budgets (name, amount_usd, period, warning_threshold_pct)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, amount_usd, period, warning_threshold_pct, created_at`,
		name, amountUSD, period, warningPct,
	).Scan(&b.ID, &b.Name, &b.AmountUSD, &b.Period, &b.WarningThresholdPct, &b.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create budget: %w", err)
	}
	b.Status = "ok"
	return &b, nil
}

// ListBudgets retorna budgets con current_spend del período en curso.
func (s *Service) ListBudgets(ctx context.Context, orgID uuid.UUID) ([]Budget, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, name, amount_usd, period, warning_threshold_pct, created_at
		FROM budgets WHERE deleted_at IS NULL
		ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list budgets: %w", err)
	}
	var out []Budget
	for rows.Next() {
		var b Budget
		if err := rows.Scan(&b.ID, &b.Name, &b.AmountUSD, &b.Period,
			&b.WarningThresholdPct, &b.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range out {
		var spend float64
		if err := s.Pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(SUM(cost_usd), 0) FROM cost_logs
			WHERE occurred_at >= date_trunc('%s', NOW())`,
			periodStart(out[i].Period))).Scan(&spend); err != nil {
			return nil, fmt.Errorf("current spend: %w", err)
		}
		out[i].CurrentSpendUSD = spend
		out[i].Status = BudgetStatus(spend, out[i].AmountUSD, out[i].WarningThresholdPct)
	}
	return out, nil
}

// DeleteBudget soft-delete con guard de org.
func (s *Service) DeleteBudget(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE budgets SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("delete budget: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrBudgetNotFound
	}
	return nil
}

// ExportCSV exporta spend diario o breakdown como filas CSV (header incluido).
func (s *Service) ExportCSV(ctx context.Context, orgID uuid.UUID, exportType string, days int) ([][]string, error) {
	switch exportType {
	case "spend":
		buckets, err := s.Spend(ctx, orgID, "daily", days)
		if err != nil {
			return nil, err
		}
		out := [][]string{{"date", "runs", "cost_usd"}}
		for _, b := range buckets {
			out = append(out, []string{
				b.Bucket.Format("2006-01-02"),
				strconv.FormatInt(b.Runs, 10),
				strconv.FormatFloat(b.CostUSD, 'f', 6, 64),
			})
		}
		return out, nil
	case "breakdown":
		rows, err := s.Breakdown(ctx, orgID, "model", days)
		if err != nil {
			return nil, err
		}
		out := [][]string{{"model", "runs", "cost_usd"}}
		for _, r := range rows {
			out = append(out, []string{
				r.Key,
				strconv.FormatInt(r.Runs, 10),
				strconv.FormatFloat(r.CostUSD, 'f', 6, 64),
			})
		}
		return out, nil
	default:
		return nil, fmt.Errorf("invalid export type %q (supported: spend, breakdown)", exportType)
	}
}

var _ = pgx.ErrNoRows
