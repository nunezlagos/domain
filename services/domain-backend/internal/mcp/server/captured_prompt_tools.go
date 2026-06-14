// Tools MCP de captured_prompts (REQ-41): persistir el raw_text del usuario
// para análisis posterior. Prefijo `domain_prompt_capture` + listing.
//
// El LLM cliente debe llamar domain_prompt_capture UNA vez por turn
// (apenas reciba el mensaje del user), pasando content + opcionalmente
// session_id + project_slug. char_count se computa server-side como
// proxy de tokens hasta tener integración real.
package mcpserver

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	capturedpromptsvc "nunezlagos/domain/internal/service/capturedprompt"
)

func registerCapturedPromptTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	// captured_prompts tiene RLS FORCE (mig 000104): wrap con tx + SET LOCAL.
	rls := func(h mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, h)
	}
	return []mcpgo.ServerTool{
		{Tool: toolPromptCapture(), Handler: wrap.Wrap("domain_prompt_capture", rls(deps.handlePromptCapture))},
		{Tool: toolPromptCapturedList(), Handler: wrap.Wrap("domain_prompt_captured_list", rls(deps.handlePromptCapturedList))},
	}
}

func toolPromptCapture() mcp.Tool {
	return mcp.NewTool("domain_prompt_capture",
		mcp.WithDescription("Persiste el raw_text que el usuario escribió en este turn. Llamar UNA vez al inicio de cada turn, antes de actuar. char_count se computa server-side (proxy de tokens)."),
		mcp.WithString("content",
			mcp.Description("Texto plano del mensaje del usuario, tal cual lo escribió."),
			mcp.Required(),
		),
		mcp.WithString("session_id",
			mcp.Description("UUID de la session activa (opcional). Si se da, el prompt queda ligado a esa sesión para análisis temporal."),
		),
		mcp.WithString("project_slug",
			mcp.Description("Slug del proyecto en cuyo contexto se mandó el mensaje (opcional)."),
		),
		mcp.WithString("client_kind",
			mcp.Description("Cliente IDE que captura: claude-code | opencode | cursor | cline | continue | claude-desktop."),
		),
		mcp.WithString("model",
			mcp.Description("Modelo LLM en uso (ej. claude-opus-4-7). Opcional."),
		),
	)
}

func toolPromptCapturedList() mcp.Tool {
	return mcp.NewTool("domain_prompt_captured_list",
		mcp.WithDescription("Lista prompts del usuario capturados, filtrables por session/project/user. Para revisión y análisis."),
		mcp.WithString("session_id",
			mcp.Description("Filtrar por session_id (UUID)"),
		),
		mcp.WithString("project_slug",
			mcp.Description("Filtrar por slug de proyecto"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Máx resultados (default 50, max 200)"),
		),
		mcp.WithNumber("offset",
			mcp.Description("Offset paginación (default 0)"),
		),
	)
}

func (d *Deps) handlePromptCapture(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if d.CapturedPrompts == nil {
		return mcp.NewToolResultError("captured_prompts service not configured"), nil
	}
	args := req.GetArguments()
	content, _ := args["content"].(string)
	if content == "" {
		return mcp.NewToolResultError("content requerido"), nil
	}
	orgID, err := uuid.Parse(d.Principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	userID, err := uuid.Parse(d.Principal.UserID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal user_id"), nil
	}

	in := capturedpromptsvc.CaptureInput{
		OrganizationID: orgID,
		UserID:         userID,
		Content:        content,
	}
	if v, ok := args["session_id"].(string); ok && v != "" {
		if sid, perr := uuid.Parse(v); perr == nil {
			in.SessionID = &sid
		}
	}
	if v, ok := args["project_slug"].(string); ok && v != "" && d.Projects != nil {
		if proj, perr := d.Projects.GetBySlug(ctx, orgID, v); perr == nil && proj != nil {
			pid := proj.ID
			in.ProjectID = &pid
		}
	}
	if v, ok := args["client_kind"].(string); ok {
		in.ClientKind = v
	}
	if v, ok := args["model"].(string); ok {
		in.Model = v
	}

	p, err := d.CapturedPrompts.Capture(ctx, in)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("capture failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"id":         p.ID,
		"char_count": p.CharCount,
		"captured":   true,
	})
}

func (d *Deps) handlePromptCapturedList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if d.CapturedPrompts == nil {
		return mcp.NewToolResultError("captured_prompts service not configured"), nil
	}
	orgID, err := uuid.Parse(d.Principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	args := req.GetArguments()
	filter := capturedpromptsvc.ListFilter{}
	if v, ok := args["session_id"].(string); ok && v != "" {
		if sid, perr := uuid.Parse(v); perr == nil {
			filter.SessionID = &sid
		}
	}
	if v, ok := args["project_slug"].(string); ok && v != "" && d.Projects != nil {
		if proj, perr := d.Projects.GetBySlug(ctx, orgID, v); perr == nil && proj != nil {
			pid := proj.ID
			filter.ProjectID = &pid
		}
	}
	if v, ok := args["limit"].(float64); ok {
		filter.Limit = int(v)
	}
	if v, ok := args["offset"].(float64); ok {
		filter.Offset = int(v)
	}

	list, total, err := d.CapturedPrompts.List(ctx, orgID, filter)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"prompts": list,
		"total":   total,
	})
}
