package mcpserver

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/observability"
)

// workflowTraceStore es la abstraccion minima para que los tools de
// observability lean workflows+workflow_steps. Se inyecta via Deps.
// Implementacion default es observability.PGWorkflowStore; tests pueden
// inyectar un fake.
type workflowTraceStore interface {
	GetWorkflow(ctx context.Context, id uuid.UUID) (observability.WorkflowRow, error)
}

type workflowTraceHandlers struct {
	store workflowTraceStore
}

// NewWorkflowTraceHandlers construye el handler con un store.
// Si store es nil, los tools devuelven error explicito al invocarse.
func NewWorkflowTraceHandlers(pool *pgxpool.Pool) *workflowTraceHandlers {
	if pool == nil {
		return &workflowTraceHandlers{}
	}
	return &workflowTraceHandlers{store: &observability.PGWorkflowStore{Pool: pool}}
}

func toolWorkflowTrace() mcp.Tool {
	return mcp.NewTool("domain_workflow_trace",
		mcp.WithDescription("Devuelve el arbol cronologico de steps (tool/fn/sql/http) de un workflow dado su workflow_id (uuid v7)."),
		mcp.WithString("workflow_id",
			mcp.Description("UUID v7 del workflow (workflow_id retornado en logs de tool invocations o headers X-Workflow-Id)"),
			mcp.Required(),
		),
	)
}

func toolWorkflowRecent() mcp.Tool {
	return mcp.NewTool("domain_workflow_recent",
		mcp.WithDescription("Lista workflows ordenados por last_activity_at DESC. Soporta filtro opcional por project_slug y ventana temporal."),
		mcp.WithString("since",
			mcp.Description("ISO timestamp desde cuando (default: ultimo 24h)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximo de workflows a retornar (default 20, max 100)"),
		),
		mcp.WithString("project_slug",
			mcp.Description("Filtrar por project_id resuelto del slug (opcional)"),
		),
	)
}

func toolWorkflowSlowest() mcp.Tool {
	return mcp.NewTool("domain_workflow_slowest",
		mcp.WithDescription("Top N workflows por total_duration_ms en la ventana temporal (default 24h, max 100)."),
		mcp.WithString("since",
			mcp.Description("ISO timestamp desde cuando (default: ultimo 24h)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximo de workflows (default 10, max 100)"),
		),
	)
}

func (h *workflowTraceHandlers) handleWorkflowTrace(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.store == nil {
		return mcp.NewToolResultError("workflow trace store not configured"), nil
	}
	idStr, err := req.RequireString("workflow_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("invalid workflow_id: " + err.Error()), nil
	}
	wf, err := h.store.GetWorkflow(ctx, id)
	if err != nil {
		return mcp.NewToolResultError("get workflow: " + err.Error()), nil
	}
	body, _ := json.MarshalIndent(map[string]any{
		"workflow": workflowRowToMap(wf),
	}, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(string(body))}}, nil
}

func (h *workflowTraceHandlers) handleWorkflowRecent(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pool, ok := h.store.(*observability.PGWorkflowStore)
	if !ok || pool == nil || pool.Pool == nil {
		return mcp.NewToolResultError("workflow trace store not configured (recent requires PG)"), nil
	}
	since := req.GetString("since", "")
	limit := req.GetInt("limit", 20)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	sinceAt, _ := parseSinceOrDefault(since, 24*time.Hour)

	rows, err := pool.Pool.Query(ctx, `
		SELECT id, name, status, started_at, ended_at,
			total_tool_calls, total_errors, total_duration_ms,
			actor_id, api_key_id, project_id, last_activity_at
		FROM workflows
		WHERE last_activity_at >= $1
		ORDER BY last_activity_at DESC
		LIMIT $2
	`, sinceAt, limit)
	if err != nil {
		return mcp.NewToolResultError("query: " + err.Error()), nil
	}
	defer rows.Close()

	out := make([]map[string]any, 0, limit)
	for rows.Next() {
		w, err := scanWorkflowRow(rows)
		if err != nil {
			return mcp.NewToolResultError("scan: " + err.Error()), nil
		}
		out = append(out, workflowRowToMap(w))
	}
	body, _ := json.MarshalIndent(map[string]any{
		"workflows": out,
		"count":     len(out),
		"since":     sinceAt,
	}, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(string(body))}}, nil
}

func (h *workflowTraceHandlers) handleWorkflowSlowest(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pool, ok := h.store.(*observability.PGWorkflowStore)
	if !ok || pool == nil || pool.Pool == nil {
		return mcp.NewToolResultError("workflow trace store not configured (slowest requires PG)"), nil
	}
	since := req.GetString("since", "")
	limit := req.GetInt("limit", 10)
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	sinceAt, _ := parseSinceOrDefault(since, 24*time.Hour)

	rows, err := pool.Pool.Query(ctx, `
		SELECT id, name, status, started_at, ended_at,
			total_tool_calls, total_errors, total_duration_ms,
			actor_id, api_key_id, project_id, last_activity_at
		FROM workflows
		WHERE last_activity_at >= $1 AND total_duration_ms > 0
		ORDER BY total_duration_ms DESC
		LIMIT $2
	`, sinceAt, limit)
	if err != nil {
		return mcp.NewToolResultError("query: " + err.Error()), nil
	}
	defer rows.Close()

	out := make([]map[string]any, 0, limit)
	for rows.Next() {
		w, err := scanWorkflowRow(rows)
		if err != nil {
			return mcp.NewToolResultError("scan: " + err.Error()), nil
		}
		out = append(out, workflowRowToMap(w))
	}
	body, _ := json.MarshalIndent(map[string]any{
		"workflows": out,
		"count":     len(out),
		"since":     sinceAt,
	}, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(string(body))}}, nil
}

func workflowRowToMap(w observability.WorkflowRow) map[string]any {
	return map[string]any{
		"id":                w.ID,
		"name":              w.Name,
		"status":            string(w.Status),
		"started_at":        w.StartedAt,
		"ended_at":          w.EndedAt,
		"total_tool_calls":  w.TotalToolCalls,
		"total_errors":      w.TotalErrors,
		"total_duration_ms": w.TotalDurationMS,
		"actor_id":          w.ActorID,
		"api_key_id":        w.APIKeyID,
		"project_id":        w.ProjectID,
		"last_activity_at":  w.LastActivityAt,
	}
}

// scanWorkflowRow escanea un row del Query en observability.WorkflowRow.
// Helper comun para evitar duplicar la signature SQL.
func scanWorkflowRow(rows interface {
	Scan(dest ...any) error
}) (observability.WorkflowRow, error) {
	var (
		w       observability.WorkflowRow
		status  string
		name    *string
		actor   *uuid.UUID
		apiKey  *uuid.UUID
		project *uuid.UUID
	)
	err := rows.Scan(&w.ID, &name, &status, &w.StartedAt, &w.EndedAt,
		&w.TotalToolCalls, &w.TotalErrors, &w.TotalDurationMS,
		&actor, &apiKey, &project, &w.LastActivityAt)
	if err != nil {
		return observability.WorkflowRow{}, err
	}
	w.Status = observability.WorkflowStatus(status)
	if name != nil {
		w.Name = *name
	}
	if actor != nil {
		w.ActorID = *actor
	}
	if apiKey != nil {
		w.APIKeyID = *apiKey
	}
	if project != nil {
		w.ProjectID = *project
	}
	return w, nil
}

func parseSinceOrDefault(since string, defaultWindow time.Duration) (time.Time, error) {
	if since == "" {
		return time.Now().Add(-defaultWindow), nil
	}
	t, err := time.Parse(time.RFC3339, since)
	if err != nil {
		return time.Now().Add(-defaultWindow), nil
	}
	return t, nil
}

func registerWorkflowTraceTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := NewWorkflowTraceHandlers(deps.Pool)
	return []mcpgo.ServerTool{
		{Tool: toolWorkflowTrace(), Handler: wrap.Wrap("domain_workflow_trace", h.handleWorkflowTrace)},
		{Tool: toolWorkflowRecent(), Handler: wrap.Wrap("domain_workflow_recent", h.handleWorkflowRecent)},
		{Tool: toolWorkflowSlowest(), Handler: wrap.Wrap("domain_workflow_slowest", h.handleWorkflowSlowest)},
	}
}