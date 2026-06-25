// REQ-53 — domain_health: autodiagnostico ligero del estado de domain
// desde la perspectiva del cliente. Permite al user (o al LLM) detectar
// problemas de conectividad/config sin tener que probar mil tools.
//
// Devuelve: estado de auth, version de schema_migrations, count de
// proyectos, ultimo project_session_bootstrap del usuario, tools
// disponibles y servicios opcionales no configurados.
package mcpserver

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/store/txctx"
)

type healthHandlers struct {
	extSync        any
	orchestrator   any
	promptRouter   any
	workflowImport any
	pool           *pgxpool.Pool
	principal      *apikey.Principal
}

func (h *healthHandlers) q(ctx context.Context) interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
} {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return tx
	}
	return h.pool
}

func registerHealthTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := &healthHandlers{
		extSync:        deps.ExtSync,
		orchestrator:   deps.Orchestrator,
		promptRouter:   deps.PromptRouter,
		workflowImport: deps.WorkflowImport,
		pool:           deps.Pool,
		principal:      deps.Principal,
	}
	rls := func(fn mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, fn)
	}
	return []mcpgo.ServerTool{
		{Tool: toolHealth(), Handler: wrap.Wrap("domain_health", rls(h.handleHealth))},
	}
}

func toolHealth() mcp.Tool {
	return mcp.NewTool("domain_health",
		mcp.WithDescription("Autodiagnostico de domain. Devuelve estado de auth, schema version, counts de objetos del usuario y servicios opcionales no configurados. Llamar cuando algo no funcione 'sin razon obvia' o como sanity check post-install."),
	)
}

func (h *healthHandlers) handleHealth(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resp := map[string]any{
		"server": map[string]any{
			"name":       "domain-mcp",
			"go_version": runtime.Version(),
			"checked_at": time.Now().UTC().Format(time.RFC3339),
		},
	}

	if h.principal == nil {
		resp["auth"] = map[string]any{"ok": false, "reason": "no principal — set DOMAIN_API_KEY"}
		return toolResultJSON(resp)
	}
	resp["auth"] = map[string]any{
		"ok":         true,
		"user_id":    h.principal.UserID,
		"org_id":     h.principal.OrganizationID,
		"role":       h.principal.Role,
		"api_key_id": h.principal.APIKeyID,
	}

	if h.pool == nil {
		resp["db"] = map[string]any{"ok": false, "reason": "pool nil"}
		return toolResultJSON(resp)
	}

	var schemaVer int
	var dirty bool
	dbStatus := map[string]any{"ok": true}
	if err := h.q(ctx).QueryRow(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&schemaVer, &dirty); err != nil {
		dbStatus["ok"] = false
		dbStatus["reason"] = err.Error()
	} else {
		dbStatus["schema_version"] = schemaVer
		dbStatus["dirty"] = dirty
	}
	resp["db"] = dbStatus

	userID, _ := uuid.Parse(h.principal.UserID)

	counts := map[string]any{}
	queries := []struct {
		key string
		sql string
	}{
		{"projects", "SELECT COUNT(*) FROM projects WHERE deleted_at IS NULL"},
		{"clients", "SELECT COUNT(*) FROM project_clients WHERE deleted_at IS NULL"},
		{"tickets", "SELECT COUNT(*) FROM project_tickets WHERE deleted_at IS NULL"},
		{"observations", "SELECT COUNT(*) FROM knowledge_observations WHERE deleted_at IS NULL"},
		{"crons", "SELECT COUNT(*) FROM crons WHERE deleted_at IS NULL"},
		{"proposals_pending", "SELECT COUNT(*) FROM project_policies WHERE proposed=true AND deleted_at IS NULL"},
		{"verifications_open", "SELECT COUNT(*) FROM tdd_verifications WHERE status IN ('pending','running','failed','partial')"},
	}
	for _, q := range queries {
		var n int
		if err := h.q(ctx).QueryRow(ctx, q.sql).Scan(&n); err == nil {
			counts[q.key] = n
		} else {
			counts[q.key] = nil
		}
	}

	var promptCount int
	if err := h.q(ctx).QueryRow(ctx,
		"SELECT COUNT(*) FROM prompt_captured WHERE user_id=$1",
		userID,
	).Scan(&promptCount); err == nil {
		counts["my_captured_prompts"] = promptCount
	}
	resp["counts"] = counts

	var lastSlug, lastSeenStr string
	if err := h.q(ctx).QueryRow(ctx,
		`SELECT slug, to_char(last_seen_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		   FROM projects
		   WHERE last_seen_at IS NOT NULL
		   ORDER BY last_seen_at DESC LIMIT 1`,
	).Scan(&lastSlug, &lastSeenStr); err == nil {
		resp["last_active_project"] = map[string]any{
			"slug":         lastSlug,
			"last_seen_at": lastSeenStr,
		}
	}

	missing := []string{}
	if h.promptRouter == nil {
		missing = append(missing, "prompt_router")
	}
	if h.extSync == nil {
		missing = append(missing, "ext_sync")
	}
	if h.orchestrator == nil {
		missing = append(missing, "orchestrator")
	}
	if h.workflowImport == nil {
		missing = append(missing, "workflow_import")
	}
	resp["optional_services_not_configured"] = missing

	ok := true
	if dbStatus["ok"] != true {
		ok = false
	}
	if dirty {
		ok = false
	}
	resp["healthy"] = ok
	if !ok {
		resp["hint"] = "DB unreachable o migration dirty — chequear logs del backend."
	}

	return toolResultJSON(resp)
}

// silenciar imports si no se usan en otros tools
var _ = fmt.Sprintf
