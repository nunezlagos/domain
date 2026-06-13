// Package usage — issue-33.4 quota snapshot read-only para dashboard.
//
// Expone Current (snapshot del día UTC actual) e History (agregación diaria
// últimos N días) sin modificar estado. El filtro por org es obligatorio y
// SIEMPRE proviene del principal autenticado (no de query params).
//
// Phase 1: queries directas con GROUP BY. La materialized view propuesta en
// design.md queda como optimización futura (migration 000098) si las
// queries no escalan; con los índices existentes (cost_logs_org_occurred_idx,
// agent_runs_agent_idx, flow_runs_flow_idx) la perf objetivo (<500ms a
// 1M cost_logs / 30d) es alcanzable sin matview.
package usage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/store/txctx"
)

// ErrInvalidDays se devuelve cuando days está fuera del rango permitido.
var ErrInvalidDays = errors.New("days must be between 1 and 365")

// ErrOrgNotFound se devuelve cuando la org del principal no existe (deleted).
var ErrOrgNotFound = errors.New("organization not found")

const (
	defaultRateLimitPerMin = 1000
	defaultMaxFlowDuration = 300
	historyDefaultDays     = 7
	historyMaxDays         = 365
)

// Snapshot es la respuesta de Current.
type Snapshot struct {
	Organization OrgRef   `json:"organization"`
	Period       Period   `json:"period"`
	Counters     Counters `json:"counters"`
	Limits       Limits   `json:"limits"`
}

// OrgRef identidad mínima de la org en respuestas.
type OrgRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// Period ventana [Start, End) del snapshot.
type Period struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// Counters contadores del día UTC actual.
type Counters struct {
	Observations   int64   `json:"observations"`
	Agents         int64   `json:"agents"`
	AgentRunsToday int64   `json:"agent_runs_today"`
	FlowRunsToday  int64   `json:"flow_runs_today"`
	CostUSDToday   float64 `json:"cost_usd_today"`
	TokensInToday  int64   `json:"tokens_in_today"`
	TokensOutToday int64   `json:"tokens_out_today"`
}

// Limits cuotas vigentes para la org.
type Limits struct {
	RateLimitPerMinute     int `json:"rate_limit_per_minute"`
	MaxFlowDurationSeconds int `json:"max_flow_duration_seconds"`
}

// DayAggregate fila histórica (un día).
type DayAggregate struct {
	Date         string  `json:"date"`
	Observations int64   `json:"observations"`
	CostUSD      float64 `json:"cost_usd"`
	AgentRuns    int64   `json:"agent_runs"`
	FlowRuns     int64   `json:"flow_runs"`
}

// History respuesta de History.
type History struct {
	Organization OrgRef         `json:"organization"`
	History      []DayAggregate `json:"history"`
}

// Service consulta agregados de uso para una org.
//
// Pool debe ser pools.App (NOBYPASSRLS): observations tiene RLS FORCE y se
// consulta dentro de txctx.WithOrgTx; el resto de tablas filtra por
// organization_id explícito en WHERE.
type Service struct {
	Pool                   *pgxpool.Pool
	DefaultRateLimitPerMin int
	DefaultMaxFlowDuration int
	Now                    func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) rateLimit() int {
	if s.DefaultRateLimitPerMin > 0 {
		return s.DefaultRateLimitPerMin
	}
	return defaultRateLimitPerMin
}

func (s *Service) maxFlowDuration() int {
	if s.DefaultMaxFlowDuration > 0 {
		return s.DefaultMaxFlowDuration
	}
	return defaultMaxFlowDuration
}

// dayWindow retorna [start, end) del día UTC actual.
func (s *Service) dayWindow() (start, end time.Time) {
	now := s.now()
	start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	end = start.Add(24 * time.Hour)
	return
}

// Current calcula el snapshot del día UTC actual.
func (s *Service) Current(ctx context.Context, orgID uuid.UUID) (*Snapshot, error) {
	if orgID == uuid.Nil {
		return nil, fmt.Errorf("orgID required")
	}
	start, end := s.dayWindow()

	snap := &Snapshot{
		Period: Period{Start: start, End: end},
		Limits: Limits{
			RateLimitPerMinute:     s.rateLimit(),
			MaxFlowDurationSeconds: s.maxFlowDuration(),
		},
	}

	err := s.Pool.QueryRow(ctx,
		`SELECT id::text, name, slug FROM organizations
		 WHERE id = $1 AND deleted_at IS NULL`,
		orgID,
	).Scan(&snap.Organization.ID, &snap.Organization.Name, &snap.Organization.Slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrgNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get org: %w", err)
	}

	if err := txctx.WithOrgTx(ctx, s.Pool, orgID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT
			  (SELECT COUNT(*) FROM observations
			     WHERE created_at >= $1 AND created_at < $2 AND deleted_at IS NULL),
			  (SELECT COUNT(*) FROM agents
			     WHERE organization_id = $3 AND deleted_at IS NULL),
			  (SELECT COUNT(*) FROM agent_runs
			     WHERE organization_id = $3 AND created_at >= $1 AND created_at < $2),
			  (SELECT COUNT(*) FROM flow_runs
			     WHERE organization_id = $3 AND created_at >= $1 AND created_at < $2),
			  (SELECT COALESCE(SUM(cost_usd), 0)::float8 FROM cost_logs
			     WHERE organization_id = $3 AND occurred_at >= $1 AND occurred_at < $2),
			  (SELECT COALESCE(SUM(tokens_input), 0)::bigint FROM cost_logs
			     WHERE organization_id = $3 AND occurred_at >= $1 AND occurred_at < $2),
			  (SELECT COALESCE(SUM(tokens_output), 0)::bigint FROM cost_logs
			     WHERE organization_id = $3 AND occurred_at >= $1 AND occurred_at < $2)
		`, start, end, orgID).Scan(
			&snap.Counters.Observations,
			&snap.Counters.Agents,
			&snap.Counters.AgentRunsToday,
			&snap.Counters.FlowRunsToday,
			&snap.Counters.CostUSDToday,
			&snap.Counters.TokensInToday,
			&snap.Counters.TokensOutToday,
		)
	}); err != nil {
		return nil, fmt.Errorf("counters query: %w", err)
	}

	var maxDur int
	err = s.Pool.QueryRow(ctx,
		`SELECT max_flow_duration_seconds FROM org_flow_config WHERE organization_id = $1`,
		orgID,
	).Scan(&maxDur)
	if err == nil {
		snap.Limits.MaxFlowDurationSeconds = maxDur
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("get org_flow_config: %w", err)
	}

	return snap, nil
}

// History agrega counters por día UTC los últimos `days` días.
// Día más reciente primero. Días sin actividad aparecen con 0s (gap-fill).
func (s *Service) History(ctx context.Context, orgID uuid.UUID, days int) (*History, error) {
	if orgID == uuid.Nil {
		return nil, fmt.Errorf("orgID required")
	}
	if days == 0 {
		days = historyDefaultDays
	}
	if days < 1 || days > historyMaxDays {
		return nil, ErrInvalidDays
	}

	now := s.now()
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Add(24 * time.Hour)
	start := end.AddDate(0, 0, -days)

	h := &History{}
	err := s.Pool.QueryRow(ctx,
		`SELECT id::text, name, slug FROM organizations
		 WHERE id = $1 AND deleted_at IS NULL`,
		orgID,
	).Scan(&h.Organization.ID, &h.Organization.Name, &h.Organization.Slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrgNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get org: %w", err)
	}

	rows, err := s.Pool.Query(ctx, `
		WITH series AS (
		  SELECT generate_series($2::timestamptz, $3::timestamptz - interval '1 day', interval '1 day')::date AS day
		),
		cost AS (
		  SELECT date_trunc('day', occurred_at AT TIME ZONE 'UTC')::date AS day,
		         SUM(cost_usd)::float8 AS cost_usd
		  FROM cost_logs
		  WHERE organization_id = $1 AND occurred_at >= $2 AND occurred_at < $3
		  GROUP BY 1
		),
		ags AS (
		  SELECT date_trunc('day', created_at AT TIME ZONE 'UTC')::date AS day, COUNT(*) AS n
		  FROM agent_runs
		  WHERE organization_id = $1 AND created_at >= $2 AND created_at < $3
		  GROUP BY 1
		),
		flw AS (
		  SELECT date_trunc('day', created_at AT TIME ZONE 'UTC')::date AS day, COUNT(*) AS n
		  FROM flow_runs
		  WHERE organization_id = $1 AND created_at >= $2 AND created_at < $3
		  GROUP BY 1
		)
		SELECT s.day,
		       COALESCE(c.cost_usd, 0)::float8 AS cost_usd,
		       COALESCE(a.n, 0)::bigint AS agent_runs,
		       COALESCE(f.n, 0)::bigint AS flow_runs
		FROM series s
		LEFT JOIN cost c ON c.day = s.day
		LEFT JOIN ags a ON a.day = s.day
		LEFT JOIN flw f ON f.day = s.day
		ORDER BY s.day DESC
	`, orgID, start, end)
	if err != nil {
		return nil, fmt.Errorf("history query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var d DayAggregate
		var t time.Time
		if err := rows.Scan(&t, &d.CostUSD, &d.AgentRuns, &d.FlowRuns); err != nil {
			return nil, fmt.Errorf("scan history row: %w", err)
		}
		d.Date = t.UTC().Format("2006-01-02")
		h.History = append(h.History, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("history rows: %w", err)
	}

	obsByDay := make(map[string]int64, len(h.History))
	if err := txctx.WithOrgTx(ctx, s.Pool, orgID, func(tx pgx.Tx) error {
		rs, qerr := tx.Query(ctx, `
			SELECT date_trunc('day', created_at AT TIME ZONE 'UTC')::date AS day,
			       COUNT(*)::bigint
			FROM observations
			WHERE created_at >= $1 AND created_at < $2 AND deleted_at IS NULL
			GROUP BY 1
		`, start, end)
		if qerr != nil {
			return qerr
		}
		defer rs.Close()
		for rs.Next() {
			var t time.Time
			var n int64
			if e := rs.Scan(&t, &n); e != nil {
				return e
			}
			obsByDay[t.UTC().Format("2006-01-02")] = n
		}
		return rs.Err()
	}); err != nil {
		return nil, fmt.Errorf("observations history: %w", err)
	}

	for i := range h.History {
		h.History[i].Observations = obsByDay[h.History[i].Date]
	}

	return h, nil
}
