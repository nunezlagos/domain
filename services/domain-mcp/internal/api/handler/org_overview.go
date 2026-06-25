










package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/store/txctx"
)

// TopUserEntry: un user con sus metricas del mes.
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

// queryRunner es la abstraccion minima de pgx.Tx + pgxpool.Pool para
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

// connRunner: wrapper pgx.Tx que libera la conexion del pool al finalizar.
// Necesario porque txctx.TxFromContext comparte la misma tx entre goroutines
// (no goroutine-safe) — abrimos tx propia por goroutine.
type connRunner struct {
	tx   pgx.Tx
	conn *pgxpool.Conn
}

func (c *connRunner) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return c.tx.QueryRow(ctx, sql, args...)
}
func (c *connRunner) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return c.tx.Query(ctx, sql, args...)
}
func (c *connRunner) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return c.tx.Exec(ctx, sql, args...)
}
func (c *connRunner) Close(ctx context.Context) {
	_ = c.tx.Rollback(ctx)
	c.conn.Release()
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




	hasTx := txctx.TxFromContext(ctx) != nil
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(4) // 3 queries + 1 DB health, paralelas



	openRunner := func() (queryRunner, error) {
		if hasTx {

			conn, err := a.Pool.Acquire(gctx)
			if err != nil {
				return nil, fmt.Errorf("acquire conn: %w", err)
			}
			tx, err := conn.BeginTx(gctx, pgx.TxOptions{})
			if err != nil {
				conn.Release()
				return nil, fmt.Errorf("begin tx: %w", err)
			}

			if _, err := tx.Exec(gctx, "SELECT set_config('app.current_org_id', $1, true)", targetOrgID.String()); err != nil {
				_ = tx.Rollback(gctx)
				conn.Release()
				return nil, fmt.Errorf("set_config: %w", err)
			}

			return &connRunner{tx: tx, conn: conn}, nil
		}
		return a.Pool, nil
	}
	_ = openRunner // se usa en cada goroutine (defer release)

	var (
		stats          OrgOverviewStats
		topUsers       []TopUserEntry
		recentActivity []RecentActivityEntry
		dbHealth       = "ok"
	)


	g.Go(func() error {
		runner, err := openRunner()
		if err != nil {
			return err
		}
		if cr, ok := runner.(*connRunner); ok {
			defer cr.Close(gctx)
		}


		row := runner.QueryRow(gctx, `
			SELECT
			  (SELECT count(*) FROM users WHERE deleted_at IS NULL),
			  (SELECT count(*) FROM agents WHERE deleted_at IS NULL),
			  (SELECT count(*) FROM (
			    SELECT id FROM agent_runs WHERE started_at > now() - interval '24 hours'
			    UNION ALL
			    SELECT id FROM flow_runs WHERE started_at > now() - interval '24 hours'
			  ) r)
		`)
		return row.Scan(&stats.MembersActive, &stats.Agents, &stats.RunsLast24h)
	})


	g.Go(func() error {
		runner, err := openRunner()
		if err != nil {
			return err
		}
		if cr, ok := runner.(*connRunner); ok {
			defer cr.Close(gctx)
		}



		rows, err := runner.Query(gctx, `
			SELECT
			  u.id, COALESCE(u.name, ''), u.email,
			  COALESCE(p.prompts, 0)
			FROM users u
			LEFT JOIN (
			  SELECT user_id, count(*) AS prompts
			  FROM prompt_captured
			  WHERE captured_at >= date_trunc('month', now())
			  GROUP BY user_id
			) p ON p.user_id = u.id
			WHERE u.deleted_at IS NULL
			ORDER BY COALESCE(p.prompts, 0) DESC
			LIMIT 5
		`)
		if err != nil {
			return fmt.Errorf("top users query: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var u TopUserEntry
			if err := rows.Scan(&u.UserID, &u.Name, &u.Email, &u.Prompts); err != nil {
				return err
			}
			topUsers = append(topUsers, u)
		}
		return rows.Err()
	})


	g.Go(func() error {
		runner, err := openRunner()
		if err != nil {
			return err
		}
		if cr, ok := runner.(*connRunner); ok {
			defer cr.Close(gctx)
		}
		rows, err := runner.Query(gctx, `
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


	if isSuperAdmin {
		resp.SystemHealth = &SystemHealth{
			API:         "ok",
			Database:    dbHealth,
			LLMProvider: "unknown",
		}
	}

	writeJSON(w, http.StatusOK, resp)
}
