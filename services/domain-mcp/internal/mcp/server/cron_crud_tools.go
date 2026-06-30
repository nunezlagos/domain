// REQ-48 — tools CRUD de crons MCP. Hoy ya existe domain_cron_list pero
// faltaba lo mas importante: como CREARLOS, pausarlos, borrarlos y ver
// su historial de ejecuciones. El usuario crea automatizaciones
// apuntando a skills/flows/agents que ya existen (ej. skill custom
// "jira-create-issue" o flow "weekly-backup").
//
// El scheduler interno (internal/scheduler/cron) polla crons enabled
// con next_run_at vencido cada 30s y dispatcha via internal/dispatch.
// Ningun cambio aqui — solo exponemos CRUD por MCP.
package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	cronsvc "nunezlagos/domain/internal/service/cron"
)

type cronService interface {
	Create(ctx context.Context, in cronsvc.CreateInput) (*cronsvc.Cron, error)
	SetEnabled(ctx context.Context, id uuid.UUID, enabled bool) error
	SoftDelete(ctx context.Context, id, actorID uuid.UUID) error
	History(ctx context.Context, cronID uuid.UUID, limit int) ([]cronsvc.Execution, error)
}

type cronHandlers struct {
	crons     cronService
	principal *apikey.Principal
}

func registerCronCRUDTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := &cronHandlers{crons: deps.Crons, principal: deps.Principal}
	return []mcpgo.ServerTool{
		{Tool: toolCronCreate(), Handler: wrap.Wrap("domain_cron_create", h.handleCronCreate)},
		{Tool: toolCronSetEnabled(), Handler: wrap.Wrap("domain_cron_set_enabled", h.handleCronSetEnabled)},
		{Tool: toolCronDelete(), Handler: wrap.Wrap("domain_cron_delete", h.handleCronDelete)},
		{Tool: toolCronHistory(), Handler: wrap.Wrap("domain_cron_history", h.handleCronHistory)},
	}
}

func toolCronCreate() mcp.Tool {
	return mcp.NewTool("domain_cron_create",
		mcp.WithDescription("Crea un cron que dispara periodicamente un flow/agent/skill existente. El scheduler interno polla cada 30s y ejecuta lo que este vencido. Validacion: cron_expression debe ser un crontab valido (5 campos UNIX, ej '0 9 * * MON-FRI' = 9am lun-vie). Para automatizaciones concretas (email/jira/backup), crear primero un skill custom con domain_project_skill_register y apuntar el cron a ese skill_id."),
		mcp.WithString("slug", mcp.Description("Slug unico del cron en la org (kebab-case)"), mcp.Required()),
		mcp.WithString("name", mcp.Description("Nombre legible"), mcp.Required()),
		mcp.WithString("description", mcp.Description("Para que sirve este cron (1-2 lineas)")),
		mcp.WithString("cron_expression", mcp.Description("Crontab UNIX 5-campos (ej: '0 9 * * MON-FRI', '*/15 * * * *', '0 0 1 * *')"), mcp.Required()),
		mcp.WithString("timezone", mcp.Description("IANA timezone (ej: 'America/Santiago'). Default: UTC")),
		mcp.WithString("target_type", mcp.Description("Que disparar: flow | agent | skill"), mcp.Required()),
		mcp.WithString("target_id", mcp.Description("UUID del flow/agent/skill a ejecutar"), mcp.Required()),
		mcp.WithObject("inputs", mcp.Description("Inputs JSON pasados al target en cada ejecucion")),
		mcp.WithBoolean("enabled", mcp.Description("Empezar habilitado (default true). Si false, queda creado pero no se ejecuta hasta domain_cron_set_enabled.")),
	)
}

func toolCronSetEnabled() mcp.Tool {
	return mcp.NewTool("domain_cron_set_enabled",
		mcp.WithDescription("Pausa o reanuda un cron. enabled=false → el scheduler lo skipea en cada poll hasta que se reanude."),
		mcp.WithString("id", mcp.Description("UUID del cron"), mcp.Required()),
		mcp.WithBoolean("enabled", mcp.Description("true para reanudar, false para pausar"), mcp.Required()),
	)
}

func toolCronDelete() mcp.Tool {
	return mcp.NewTool("domain_cron_delete",
		mcp.WithDescription("Soft-delete de un cron. Las ejecuciones historicas se preservan; el cron deja de dispararse."),
		mcp.WithString("id", mcp.Description("UUID del cron"), mcp.Required()),
	)
}

func toolCronHistory() mcp.Tool {
	return mcp.NewTool("domain_cron_history",
		mcp.WithDescription("Historial de ejecuciones (running/completed/failed/skipped_overlap) de un cron. Para debugging y auditoria — ¿se esta disparando? ¿esta fallando?"),
		mcp.WithString("id", mcp.Description("UUID del cron"), mcp.Required()),
		mcp.WithNumber("limit", mcp.Description("Max resultados (default 20, max 100)")),
	)
}

func (h *cronHandlers) handleCronCreate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.crons == nil {
		return mcp.NewToolResultError("crons service not configured"), nil
	}
	args := req.GetArguments()
	slug, _ := args["slug"].(string)
	name, _ := args["name"].(string)
	expr, _ := args["cron_expression"].(string)
	tType, _ := args["target_type"].(string)
	tIDStr, _ := args["target_id"].(string)
	if slug == "" || name == "" || expr == "" || tType == "" || tIDStr == "" {
		return mcp.NewToolResultError("slug, name, cron_expression, target_type y target_id son requeridos"), nil
	}
	tID, err := uuid.Parse(tIDStr)
	if err != nil {
		return mcp.NewToolResultError("target_id invalido (UUID requerido)"), nil
	}
	tType = strings.ToLower(strings.TrimSpace(tType))

	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	userID, _ := uuid.Parse(h.principal.UserID)

	in := cronsvc.CreateInput{
		OrganizationID: orgID,
		CreatedBy:      &userID,
		ActorID:        userID,
		Slug:           slug,
		Name:           name,
		CronExpression: expr,
		TargetType:     tType,
		TargetID:       tID,
		Enabled:        true,
	}
	if v, ok := args["description"].(string); ok {
		in.Description = v
	}
	if v, ok := args["timezone"].(string); ok && v != "" {
		in.Timezone = v
	}
	if v, ok := args["enabled"].(bool); ok {
		in.Enabled = v
	}
	if v, ok := args["inputs"].(map[string]any); ok {
		in.Inputs = v
	}

	c, err := h.crons.Create(ctx, in)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create cron failed: %v", err)), nil
	}
	return toolResultJSON(c)
}

func (h *cronHandlers) handleCronSetEnabled(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.crons == nil {
		return mcp.NewToolResultError("crons service not configured"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id invalido"), nil
	}
	enabled, _ := args["enabled"].(bool)
	if err := h.crons.SetEnabled(ctx, id, enabled); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("set_enabled failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"id": id.String(), "enabled": enabled})
}

func (h *cronHandlers) handleCronDelete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.crons == nil {
		return mcp.NewToolResultError("crons service not configured"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id invalido"), nil
	}
	userID, _ := uuid.Parse(h.principal.UserID)
	if err := h.crons.SoftDelete(ctx, id, userID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("delete failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"id": id.String(), "deleted": true})
}

func (h *cronHandlers) handleCronHistory(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.crons == nil {
		return mcp.NewToolResultError("crons service not configured"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id invalido"), nil
	}
	limit := 20
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	execs, err := h.crons.History(ctx, id, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("history failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"executions": execs, "total": len(execs)})
}

// silenciar context si no se usa
var _ context.Context
