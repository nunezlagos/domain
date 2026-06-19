// Package usage — issue-33.4 quota snapshot read-only para dashboard.
//
// Expone Current (snapshot del día UTC actual) e History (agregación diaria
// últimos N días) sin modificar estado. El filtro por org es obligatorio y
// SIEMPRE proviene del principal autenticado (no de query params).
//
// Phase 1: queries directas con GROUP BY. La materialized view propuesta en
// design.md queda como optimización futura si las queries no escalan; con los
// índices existentes (agent_runs_agent_idx, flow_runs_flow_idx) la perf
// objetivo es alcanzable sin matview.
//
// REQ-42.2: el dominio billing/costos se eliminó; los agregados de cost_usd/
// tokens quedan en 0 (ya no se consulta ninguna tabla de costos).
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

// runInOrgTx ejecuta fn con una pgx.Tx que tiene SET LOCAL app.current_org_id
// y app.current_user_id seteados. Si ya hay una tx en ctx (inyectada por el
// middleware apikey o el wireup MCP), la reutiliza sin abrir ni commitear
// (el dueño se encarga). Si no hay tx, abre una nueva con WithOrgTx.
//
// Esto evita queries directas a tablas con RLS FORCE (organizations,
// projects, users) vía s.Pool — sin SET LOCAL, la policy filtra a 0 rows
// y aparecen falsos "not found".
func (s *Service) runInOrgTx(ctx context.Context, orgID uuid.UUID, fn func(pgx.Tx) error) error {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return fn(tx)
	}
	return txctx.WithOrgTx(ctx, s.Pool, orgID, fn)
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

	if err := s.runInOrgTx(ctx, orgID, func(tx pgx.Tx) error {
		// ISSUE-21.6 Fase D clean Round 3: tabla organizations se dropea
		// en Fase C. En single-org, hardcodeamos el OrgRef. Si orgID es
		// uuid.Nil (single-org mode), usamos un UUID fijo canónico.
		snap.Organization = singleOrgRef(orgID)

		// REQ-42.2: cost_logs se dropeó (dominio billing/costos eliminado).
		// Los counters de costo/tokens quedan en 0 (no había writer de
		// producción: la tabla siempre estuvo vacía).
		if err := tx.QueryRow(ctx, `
			SELECT
			  (SELECT COUNT(*) FROM observations
			     WHERE created_at >= $1 AND created_at < $2 AND deleted_at IS NULL),
			  (SELECT COUNT(*) FROM agents
			     WHERE deleted_at IS NULL),
			  (SELECT COUNT(*) FROM agent_runs
			     WHERE created_at >= $1 AND created_at < $2),
			  (SELECT COUNT(*) FROM flow_runs
			     WHERE created_at >= $1 AND created_at < $2)
		`, start, end).Scan(
			&snap.Counters.Observations,
			&snap.Counters.Agents,
			&snap.Counters.AgentRunsToday,
			&snap.Counters.FlowRunsToday,
		); err != nil {
			return fmt.Errorf("counters query: %w", err)
		}

		// flow_config es config global (single-org). Antes llamada
		// org_flow_config (legacy pre-Fase C); renombrada en 000146.
		var maxDur int
		err := tx.QueryRow(ctx,
			`SELECT max_flow_duration_seconds FROM flow_config LIMIT 1`,
		).Scan(&maxDur)
		if err == nil {
			snap.Limits.MaxFlowDurationSeconds = maxDur
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("get flow_config: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
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
	obsByDay := make(map[string]int64)
	if err := s.runInOrgTx(ctx, orgID, func(tx pgx.Tx) error {
		// ISSUE-21.6 Fase D clean Round 3: ver singleOrgRef() en Current().
		h.Organization = singleOrgRef(orgID)

		// REQ-42.2: cost_logs se dropeó; el agregado de cost_usd ya no se
		// consulta (CostUSD queda en 0 en cada fila).
		rows, err := tx.Query(ctx, `
			WITH series AS (
			  SELECT generate_series($1::timestamptz, $2::timestamptz - interval '1 day', interval '1 day')::date AS day
			),
			ags AS (
			  SELECT date_trunc('day', created_at AT TIME ZONE 'UTC')::date AS day, COUNT(*) AS n
			  FROM agent_runs
			  WHERE created_at >= $1 AND created_at < $2
			  GROUP BY 1
			),
			flw AS (
			  SELECT date_trunc('day', created_at AT TIME ZONE 'UTC')::date AS day, COUNT(*) AS n
			  FROM flow_runs
			  WHERE created_at >= $1 AND created_at < $2
			  GROUP BY 1
			)
			SELECT s.day,
			       COALESCE(a.n, 0)::bigint AS agent_runs,
			       COALESCE(f.n, 0)::bigint AS flow_runs
			FROM series s
			LEFT JOIN ags a ON a.day = s.day
			LEFT JOIN flw f ON f.day = s.day
			ORDER BY s.day DESC
		`, start, end)
		if err != nil {
			return fmt.Errorf("history query: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var d DayAggregate
			var t time.Time
			if err := rows.Scan(&t, &d.AgentRuns, &d.FlowRuns); err != nil {
				return fmt.Errorf("scan history row: %w", err)
			}
			d.Date = t.UTC().Format("2006-01-02")
			h.History = append(h.History, d)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("history rows: %w", err)
		}

		rs, qerr := tx.Query(ctx, `
			SELECT date_trunc('day', created_at AT TIME ZONE 'UTC')::date AS day,
			       COUNT(*)::bigint
			FROM observations
			WHERE created_at >= $1 AND created_at < $2 AND deleted_at IS NULL
			GROUP BY 1
		`, start, end)
		if qerr != nil {
			return fmt.Errorf("observations history: %w", qerr)
		}
		defer rs.Close()
		for rs.Next() {
			var t time.Time
			var n int64
			if e := rs.Scan(&t, &n); e != nil {
				return fmt.Errorf("scan observations history: %w", e)
			}
			obsByDay[t.UTC().Format("2006-01-02")] = n
		}
		return rs.Err()
	}); err != nil {
		return nil, err
	}

	for i := range h.History {
		h.History[i].Observations = obsByDay[h.History[i].Date]
	}

	return h, nil
}

// singleOrgRef retorna el OrgRef para single-org. Si el orgID que llega
// no es uuid.Nil, lo usamos tal cual (es el ID que el caller pasó);
// en otro caso (single-org mode con uuid.Nil), usamos un UUID canónico
// fijo para que los responses del API sean estables y los clients
// puedan cachearlos.
//
// ISSUE-21.6: la tabla organizations se dropea en Fase C, así que este
// helper reemplaza las queries `SELECT ... FROM organizations WHERE id = $1`.
// Pre-C: este helper nunca se llamaba (las queries reales leían la tabla).
// Post-C: este helper es la única fuente de OrgRef en respuestas.
func singleOrgRef(orgID uuid.UUID) OrgRef {
	if orgID == uuid.Nil {
		return OrgRef{
			ID:   "00000000-0000-0000-0000-000000000001",
			Name: "default",
			Slug: "default",
		}
	}
	return OrgRef{
		ID:   orgID.String(),
		Name: "default",
		Slug: "default",
	}
}
