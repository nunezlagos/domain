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

	"nunezlagos/domain/internal/service/usage/usagedb"
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

//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate

// q retorna un *usagedb.Queries que usa la tx del contexto si existe,
// o el pool directo en caso contrario.
func (s *Service) q(ctx context.Context) *usagedb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return usagedb.New(tx)
	}
	return usagedb.New(s.Pool)
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
	return txctx.WithOrgTx(ctx, s.Pool, orgID, func(tx pgx.Tx) error {
		ctx = txctx.WithTxContext(ctx, tx)
		return fn(tx)
	})
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
		snap.Organization = singleOrgRef(orgID)

		rows, err := s.q(ctx).GetCounters(ctx, usagedb.GetCountersParams{
			CreatedAt:   start,
			CreatedAt_2: end,
		})
		if err != nil {
			return fmt.Errorf("counters query: %w", err)
		}
		snap.Counters = Counters{
			Observations:   rows.Observations,
			Agents:         rows.Agents,
			AgentRunsToday: rows.AgentRuns,
			FlowRunsToday:  rows.FlowRuns,
		}

		maxDur, err := s.q(ctx).GetMaxDuration(ctx)
		if err == nil {
			snap.Limits.MaxFlowDurationSeconds = int(maxDur)
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
		h.Organization = singleOrgRef(orgID)

		q := s.q(ctx)

		runs, err := q.GetRunHistory(ctx, usagedb.GetRunHistoryParams{
			Column1: start,
			Column2: end,
		})
		if err != nil {
			return fmt.Errorf("run history: %w", err)
		}
		for _, r := range runs {
			h.History = append(h.History, DayAggregate{
				Date:      r.Day.UTC().Format("2006-01-02"),
				AgentRuns: r.AgentRuns,
				FlowRuns:  r.FlowRuns,
			})
		}

		obs, err := q.GetObservationHistory(ctx, usagedb.GetObservationHistoryParams{
			CreatedAt:   start,
			CreatedAt_2: end,
		})
		if err != nil {
			return fmt.Errorf("observations history: %w", err)
		}
		for _, o := range obs {
			obsByDay[o.Day.UTC().Format("2006-01-02")] = o.Count
		}
		return nil
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
