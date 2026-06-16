// HU-41.2: handler de GET /api/v1/admin/org-overview. Devuelve stats
// agregados de una org (members, agents, runs, tokens, cost), top 5
// users del mes, actividad reciente, y (si super_admin) system health.
//
// RBAC:
//   - admin de la org X → ve solo X
//   - super_admin → ve cualquier org (con ?org_id=)
//
// Las queries respetan RLS porque se ejecutan dentro de la tx del
// middleware de auth (issue-25.14) si está disponible; sino en el Pool.

package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/sync/errgroup"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/store/txctx"
)

// TopUserEntry: un user con sus métricas del mes.
type TopUserEntry struct {
	UserID    uuid.UUID `json:"user_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Prompts   int       `json:"prompts"`
	TokensIn  int64     `json:"tokens_in"`
	TokensOut int64     `json:"tokens_out"`
	CostUSD   float64   `json:"cost_usd"`
}

// RecentActivityEntry: un evento del audit log.
type RecentActivityEntry struct {
	Actor  string    `json:"actor"`
	Action string    `json:"action"`
	Target string    `json:"target"`
	At     time.Time `json:"at"`
}

// SystemHealth: health del sistema.
type SystemHealth struct {
	API         string `json:"api"`          // "ok" | "error"
	Database    string `json:"database"`     // "ok" | "error"
	LLMProvider string `json:"llm_provider"` // "ok" | "error" | "unknown"
}

// Stats: contadores agregados del mes.
type OrgOverviewStats struct {
	MembersActive    int     `json:"members_active"`
	Agents           int     `json:"agents"`
	RunsLast24h      int     `json:"runs_24h"`
	TokensThisMonth  int64   `json:"tokens_this_month"`
	CostThisMonthUSD float64 `json:"cost_this_month_usd"`
}

// OrgOverviewResponse: payload completo del endpoint.
type OrgOverviewResponse struct {
	OrgID             uuid.UUID             `json:"org_id"`
	Stats             OrgOverviewStats      `json:"stats"`
	TopUsersThisMonth []TopUserEntry        `json:"top_users_this_month"`
	RecentActivity    []RecentActivityEntry `json:"recent_activity"`
	SystemHealth      *SystemHealth         `json:"system_health,omitempty"`
}

// queryRunner es la abstracción mínima de pgx.Tx + pgxpool.Pool para
// que las queries corran con RLS cuando hay tx, o sin RLS en el Pool.
type queryRunner interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func runnerFromCtx(ctx context.Context, pool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}) queryRunner {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return tx
	}
	return pool
}

// ensureSlice: si el slice es nil, devuelve []Type{} para que el JSON
// marshalice como [] en vez de null.
func ensureSlice[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// getOrgOverview es el handler HTTP montado en /api/v1/admin/org-overview.
func (a *API) getOrgOverview(w http.ResponseWriter, r *http.Request) {
	principal, ok := apikey.FromContext(r.Context())
	if !ok || principal == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing principal")
		return
	}
	callerOrgID, perr := uuid.Parse(principal.OrganizationID)
	if perr != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid org_id in principal")
		return
	}
	isSuperAdmin := principal.Role == "super_admin"

	// Resolver org target. Default: org del caller.
	targetOrgID := callerOrgID
	if q := r.URL.Query().Get("org_id"); q != "" {
		parsed, qerr := uuid.Parse(q)
		if qerr != nil {
			writeError(w, http.StatusBadRequest, "invalid_org_id", "org_id must be a valid UUID")
			return
		}
		targetOrgID = parsed
	}
	if targetOrgID != callerOrgID && !isSuperAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "cannot view other org's overview")
		return
	}

	ctx := r.Context()
	// Usar el Pool directamente (cada query toma su propia conexión).
	// RLS se convierte en defense-in-depth: el filtro `WHERE organization_id`
	// en cada query es la fuente de verdad. Esto permite paralelizar
	// con errgroup sin caer en "conn busy" (misma conexión usada
	// concurrentemente).
	g, gctx := errgroup.WithContext(ctx)

	var (
		stats          OrgOverviewStats
		topUsers       []TopUserEntry
		recentActivity []RecentActivityEntry
		dbHealth       = "ok"
	)

	// Stats agregados.
	g.Go(func() error {
		row := a.Pool.QueryRow(gctx, `
			SELECT
			  (SELECT count(*) FROM users WHERE organization_id = $1 AND deleted_at IS NULL),
			  (SELECT count(*) FROM agents WHERE organization_id = $1 AND deleted_at IS NULL),
			  (SELECT count(*) FROM (
			    SELECT id FROM agent_runs WHERE organization_id = $1 AND started_at > now() - interval '24 hours'
			    UNION ALL
			    SELECT id FROM flow_runs WHERE organization_id = $1 AND started_at > now() - interval '24 hours'
			  ) r),
			  COALESCE((SELECT sum(tokens_input + tokens_output) FROM cost_logs WHERE organization_id = $1 AND occurred_at >= date_trunc('month', now())), 0)::bigint,
			  COALESCE((SELECT sum(cost_usd) FROM cost_logs WHERE organization_id = $1 AND occurred_at >= date_trunc('month', now())), 0)
		`, targetOrgID)
		return row.Scan(&stats.MembersActive, &stats.Agents, &stats.RunsLast24h, &stats.TokensThisMonth, &stats.CostThisMonthUSD)
	})

	// Top 5 users del mes (por cost USD desc).
	g.Go(func() error {
		rows, err := a.Pool.Query(gctx, `
			SELECT
			  u.id, COALESCE(u.name, ''), u.email,
			  COALESCE(p.prompts, 0),
			  COALESCE(t.tokens_in, 0),
			  COALESCE(t.tokens_out, 0),
			  COALESCE(t.cost, 0)
			FROM users u
			LEFT JOIN (
			  SELECT user_id, count(*) AS prompts
			  FROM captured_prompts
			  WHERE organization_id = $1 AND captured_at >= date_trunc('month', now())
			  GROUP BY user_id
			) p ON p.user_id = u.id
			LEFT JOIN (
			  SELECT user_id,
			         sum(tokens_input)  AS tokens_in,
			         sum(tokens_output) AS tokens_out,
			         sum(cost_usd)     AS cost
			  FROM cost_logs
			  WHERE organization_id = $1 AND occurred_at >= date_trunc('month', now())
			  GROUP BY user_id
			) t ON t.user_id = u.id
			WHERE u.organization_id = $1 AND u.deleted_at IS NULL
			ORDER BY COALESCE(t.cost, 0) DESC
			LIMIT 5
		`, targetOrgID)
		if err != nil {
			return fmt.Errorf("top users query: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var u TopUserEntry
			if err := rows.Scan(&u.UserID, &u.Name, &u.Email, &u.Prompts, &u.TokensIn, &u.TokensOut, &u.CostUSD); err != nil {
				return err
			}
			topUsers = append(topUsers, u)
		}
		return rows.Err()
	})

	// Recent activity (últimos 10 del audit_log).
	g.Go(func() error {
		rows, err := a.Pool.Query(gctx, `
			SELECT
			  COALESCE(actor.email, 'system'),
			  al.action,
			  COALESCE(al.entity_type || '/' || al.entity_id::text, ''),
			  al.occurred_at
			FROM audit_log al
			LEFT JOIN users actor ON actor.id = al.actor_id
			WHERE al.origin_org_id = $1
			ORDER BY al.occurred_at DESC
			LIMIT 10
		`, targetOrgID)
		if err != nil {
			return fmt.Errorf("recent activity query: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var e RecentActivityEntry
			if err := rows.Scan(&e.Actor, &e.Action, &e.Target, &e.At); err != nil {
				return err
			}
			recentActivity = append(recentActivity, e)
		}
		return rows.Err()
	})

	// DB health.
	g.Go(func() error {
		var ok int
		if err := a.Pool.QueryRow(gctx, `SELECT 1`).Scan(&ok); err != nil {
			dbHealth = "error"
		} else {
			dbHealth = "ok"
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		writeError(w, http.StatusInternalServerError, "query_failed", err.Error())
		return
	}

	resp := OrgOverviewResponse{
		OrgID:             targetOrgID,
		Stats:             stats,
		TopUsersThisMonth: ensureSlice(topUsers),
		RecentActivity:    ensureSlice(recentActivity),
	}

	// System health solo si super_admin.
	if isSuperAdmin {
		resp.SystemHealth = &SystemHealth{
			API:         "ok",
			Database:    dbHealth,
			LLMProvider: "unknown",
		}
	}

	writeJSON(w, http.StatusOK, resp)
}
