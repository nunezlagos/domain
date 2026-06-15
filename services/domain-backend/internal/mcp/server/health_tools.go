// REQ-53 — domain_health: autodiagnóstico ligero del estado de domain
// desde la perspectiva del cliente. Permite al user (o al LLM) detectar
// problemas de conectividad/config sin tener que probar mil tools.
//
// Devuelve: estado de auth, version de schema_migrations, count de
// proyectos, último project_session_bootstrap del usuario, tools
// disponibles y servicios opcionales no configurados.
package mcpserver

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"
)

func registerHealthTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	// Wrappear con withOrgTxHandler para que los SELECTs respeten RLS y
	// vean los rows del org del principal (app_user, no BYPASSRLS).
	rls := func(h mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, h)
	}
	return []mcpgo.ServerTool{
		{Tool: toolHealth(), Handler: wrap.Wrap("domain_health", rls(deps.handleHealth))},
	}
}

func toolHealth() mcp.Tool {
	return mcp.NewTool("domain_health",
		mcp.WithDescription("Autodiagnóstico de domain. Devuelve estado de auth, schema version, counts de objetos del usuario y servicios opcionales no configurados. Llamar cuando algo no funcione 'sin razón obvia' o como sanity check post-install."),
	)
}

func (d *Deps) handleHealth(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resp := map[string]any{
		"server": map[string]any{
			"name":       "domain-mcp",
			"go_version": runtime.Version(),
			"checked_at": time.Now().UTC().Format(time.RFC3339),
		},
	}

	// Auth
	if d.Principal == nil {
		resp["auth"] = map[string]any{"ok": false, "reason": "no principal — set DOMAIN_API_KEY"}
		return toolResultJSON(resp)
	}
	resp["auth"] = map[string]any{
		"ok":           true,
		"user_id":      d.Principal.UserID,
		"org_id":       d.Principal.OrganizationID,
		"role":         d.Principal.Role,
		"api_key_id":   d.Principal.APIKeyID,
	}

	if d.Pool == nil {
		resp["db"] = map[string]any{"ok": false, "reason": "pool nil"}
		return toolResultJSON(resp)
	}

	// Schema migration version
	var schemaVer int
	var dirty bool
	dbStatus := map[string]any{"ok": true}
	if err := d.q(ctx).QueryRow(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&schemaVer, &dirty); err != nil {
		dbStatus["ok"] = false
		dbStatus["reason"] = err.Error()
	} else {
		dbStatus["schema_version"] = schemaVer
		dbStatus["dirty"] = dirty
	}
	resp["db"] = dbStatus

	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	userID, _ := uuid.Parse(d.Principal.UserID)

	// Counts de objetos del usuario en su org (queries simples,
	// app_admin pool si está disponible para BYPASSRLS).
	counts := map[string]any{}
	queries := []struct {
		key string
		sql string
	}{
		{"projects", "SELECT COUNT(*) FROM projects WHERE organization_id=$1 AND deleted_at IS NULL"},
		{"clients", "SELECT COUNT(*) FROM clients WHERE organization_id=$1 AND deleted_at IS NULL"},
		{"tickets", "SELECT COUNT(*) FROM project_tickets WHERE organization_id=$1 AND deleted_at IS NULL"},
		{"observations", "SELECT COUNT(*) FROM observations WHERE organization_id=$1 AND deleted_at IS NULL"},
		{"sessions_open", "SELECT COUNT(*) FROM sessions WHERE organization_id=$1 AND ended_at IS NULL"},
		{"crons", "SELECT COUNT(*) FROM crons WHERE organization_id=$1 AND deleted_at IS NULL"},
		{"proposals_pending", "SELECT COUNT(*) FROM project_policies WHERE organization_id=$1 AND proposed=true AND deleted_at IS NULL"},
		{"verifications_open", "SELECT COUNT(*) FROM verifications WHERE organization_id=$1 AND status IN ('pending','running','failed','partial')"},
	}
	for _, q := range queries {
		var n int
		if err := d.q(ctx).QueryRow(ctx, q.sql, orgID).Scan(&n); err == nil {
			counts[q.key] = n
		} else {
			counts[q.key] = nil
		}
	}
	// Captured prompts del usuario (filtrado por user_id en lugar de org)
	var promptCount int
	if err := d.q(ctx).QueryRow(ctx,
		"SELECT COUNT(*) FROM captured_prompts WHERE organization_id=$1 AND user_id=$2",
		orgID, userID,
	).Scan(&promptCount); err == nil {
		counts["my_captured_prompts"] = promptCount
	}
	resp["counts"] = counts

	// Último project_session_bootstrap del usuario (proyecto más recientemente tocado)
	var lastSlug, lastSeenStr string
	if err := d.q(ctx).QueryRow(ctx,
		`SELECT slug, to_char(last_seen_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		   FROM projects
		   WHERE organization_id = $1 AND last_seen_at IS NOT NULL
		   ORDER BY last_seen_at DESC LIMIT 1`,
		orgID,
	).Scan(&lastSlug, &lastSeenStr); err == nil {
		resp["last_active_project"] = map[string]any{
			"slug":         lastSlug,
			"last_seen_at": lastSeenStr,
		}
	}

	// Servicios opcionales: detectar cuáles NO están configurados
	missing := []string{}
	if d.PromptRouter == nil {
		missing = append(missing, "prompt_router")
	}
	if d.ExtSync == nil {
		missing = append(missing, "ext_sync")
	}
	if d.Orchestrator == nil {
		missing = append(missing, "orchestrator")
	}
	if d.WorkflowImport == nil {
		missing = append(missing, "workflow_import")
	}
	resp["optional_services_not_configured"] = missing

	// Verdict global
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
