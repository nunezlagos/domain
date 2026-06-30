

















package mcpserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/store/txctx"
)

// withOrgCtx abre una tx con SET LOCAL app.current_org_id y
// app.current_user_id, e inyecta la tx en el ctx.
//
// Retorna:
//   - ctx enriquecido con la tx (repos la extraen con txctx.TxFromContext)
//   - la tx misma (por si el caller quiere Commit explicito; sino defer
//     release() hace Rollback)
//   - release func: el caller DEBE llamarla (defer release()).
//     Si el caller hizo Commit, release es no-op. Si no, Rollback.
//
// Si el principal es nil o el pool es nil, retorna (ctx, nil, noop) sin
// wireup (legacy: queries usan pool directo, RLS no aplica — util para
// tools admin que corren con app_admin BYPASSRLS).
func withOrgCtx(ctx context.Context, pool *pgxpool.Pool, principal *apikey.Principal) (context.Context, pgx.Tx, func()) {
	noop := func() {}
	if pool == nil || principal == nil {
		return ctx, nil, noop
	}
	orgID, orgErr := uuid.Parse(principal.OrganizationID)
	userID, userErr := uuid.Parse(principal.UserID)
	if orgErr != nil || userErr != nil || orgID == uuid.Nil {
		return ctx, nil, noop
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {




		_ = err
		return ctx, nil, noop
	}
	if _, err := tx.Exec(ctx,
		`SELECT set_config('app.current_org_id', $1, true), set_config('app.current_user_id', $2, true)`,
		orgID.String(), userID.String()); err != nil {
		_ = tx.Rollback(ctx)
		return ctx, nil, noop
	}
	release := func() { _ = tx.Rollback(ctx) }
	return txctx.WithTxContext(ctx, tx), tx, release
}

// withOrgTxHandler envuelve un tool handler con el wireup RLS completo
// (issue-25.14 + cierre 25.5 Tier-1): abre tx con SET LOCAL
// app.current_org_id/user_id, la inyecta en el ctx (los services la
// toman via txctx.TxFromContext) y COMMITEA si el tool termino sin
// error. Aplicar a todo tool que toque tablas con RLS FORCE
// (observations, sessions y las de 000028).
func withOrgTxHandler(d *Deps, h mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		txCtx, tx, release := withOrgCtx(ctx, d.Pool, d.Principal)
		defer release()
		result, err := h(txCtx, req)
		if tx != nil && err == nil && (result == nil || !result.IsError) {
			if cerr := tx.Commit(txCtx); cerr != nil {
				if errors.Is(cerr, pgx.ErrTxCommitRollback) {
					// tx aborted mid-flight: cualquier INSERT/UPDATE ejecutado
					// por el handler se pierde. Devolver success aca seria
					// mentir al cliente — retornamos el error de rollback
					// para que el caller sepa que la operacion NO se persisto.
					return mcp.NewToolResultError(fmt.Sprintf("transaction aborted before commit (Rollback): %v", cerr)), nil
				}
				return mcp.NewToolResultError(fmt.Sprintf("commit failed: %v", cerr)), nil
			}
		}
		return result, err
	}
}

// q retorna la tx del contexto (wireup activo) o el Pool como fallback.
// Para queries directas de los handlers MCP sobre tablas con RLS.
// Incluye Exec para INSERT/UPDATE/DELETE (REQ-45 session_bootstrap).
func (d *Deps) q(ctx context.Context) interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
} {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return tx
	}
	return d.Pool
}

// commitOrRollback es un helper de uso comun: si err es nil y tx no es nil,
// intenta Commit; si Commit falla, Rollback. Si err != nil, Rollback.
// Pensado para envolver la llamada al servicio desde el handler MCP.
func commitOrRollback(ctx context.Context, tx pgx.Tx, err error) error {
	if tx == nil {
		return err
	}
	if err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if cerr := tx.Commit(ctx); cerr != nil {
		return fmt.Errorf("commit: %w", cerr)
	}
	return nil
}
