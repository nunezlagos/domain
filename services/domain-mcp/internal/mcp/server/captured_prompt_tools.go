// Tools MCP de captured_prompts (REQ-41): persistir el raw_text del usuario
// para analisis posterior. Prefijo `domain_prompt_capture` + listing.
//
// El LLM cliente debe llamar domain_prompt_capture UNA vez por turn
// (apenas reciba el mensaje del user), pasando content + opcionalmente
// project_slug. char_count se computa server-side como proxy de tokens
// hasta tener integracion real. (REQ-42.3: session_id removido.)
package mcpserver

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	capturedpromptsvc "nunezlagos/domain/internal/service/capturedprompt"
	projsvc "nunezlagos/domain/internal/service/project"
)

type capturedPromptService interface {
	Capture(ctx context.Context, in capturedpromptsvc.CaptureInput) (*capturedpromptsvc.Prompt, error)
	List(ctx context.Context, orgID uuid.UUID, filter capturedpromptsvc.ListFilter) ([]*capturedpromptsvc.Prompt, int64, error)
	CompleteTurn(ctx context.Context, in capturedpromptsvc.CompleteTurnInput) (*capturedpromptsvc.Prompt, error)
	SummarizeByProject(ctx context.Context, orgID, projectID uuid.UUID) (*capturedpromptsvc.SessionUsage, error)
}

type capturedPromptProjectGetter interface {
	GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*projsvc.Project, error)
}

type capturedPromptHandlers struct {
	prompts   capturedPromptService
	projects  capturedPromptProjectGetter
	principal *apikey.Principal
}

func registerCapturedPromptTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := &capturedPromptHandlers{
		prompts:   deps.CapturedPrompts,
		projects:  deps.Projects,
		principal: deps.Principal,
	}
	rls := func(fn mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, fn)
	}
	return []mcpgo.ServerTool{
		{Tool: toolPromptCapture(), Handler: wrap.Wrap("domain_prompt_capture", rls(h.handlePromptCapture))},
		{Tool: toolPromptCapturedList(), Handler: wrap.Wrap("domain_prompt_captured_list", rls(h.handlePromptCapturedList))},
		{Tool: toolTurnComplete(), Handler: wrap.Wrap("domain_turn_complete", rls(h.handleTurnComplete))},
		{Tool: toolUsageSummary(), Handler: wrap.Wrap("domain_usage_summary", rls(h.handleUsageSummary))},
	}
}

func toolPromptCapture() mcp.Tool {
	return mcp.NewTool("domain_prompt_capture",
		mcp.WithDescription("Persiste el raw_text que el usuario escribio en este turn. Llamar UNA vez al inicio de cada turn, antes de actuar. char_count se computa server-side (proxy de tokens)."),
		mcp.WithString("content",
			mcp.Description("Texto plano del mensaje del usuario, tal cual lo escribio."),
			mcp.Required(),
		),
		mcp.WithString("project_slug",
			mcp.Description("Slug del proyecto en cuyo contexto se mando el mensaje (opcional)."),
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
		mcp.WithDescription("Lista prompts del usuario capturados, filtrables por project/user. Para revision y analisis."),
		mcp.WithString("project_slug",
			mcp.Description("Filtrar por slug de proyecto"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Max resultados (default 50, max 200)"),
		),
		mcp.WithNumber("offset",
			mcp.Description("Offset paginacion (default 0)"),
		),
	)
}

func (h *capturedPromptHandlers) handlePromptCapture(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.prompts == nil {
		return mcp.NewToolResultError("captured_prompts service not configured"), nil
	}
	args := req.GetArguments()
	content, _ := args["content"].(string)
	if content == "" {
		return mcp.NewToolResultError("content requerido"), nil
	}
	orgID, err := uuid.Parse(h.principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	userID, err := uuid.Parse(h.principal.UserID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal user_id"), nil
	}

	in := capturedpromptsvc.CaptureInput{
		OrganizationID: orgID,
		UserID:         userID,
		Content:        content,
	}

	if v, ok := args["project_slug"].(string); ok && v != "" && h.projects != nil {
		if proj, perr := h.projects.GetBySlug(ctx, orgID, v); perr == nil && proj != nil {
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

	p, err := h.prompts.Capture(ctx, in)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("capture failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"id":         p.ID,
		"char_count": p.CharCount,
		"captured":   true,
	})
}

func toolTurnComplete() mcp.Tool {
	return mcp.NewTool("domain_turn_complete",
		mcp.WithDescription("Cierra el turn actual reportando cuantos chars escribio el LLM en su respuesta. El server estima tokens out con ratio 4:1 y los persiste. Llamar al final de CADA turn pasando el prompt_id que devolvio domain_prompt_capture. Permite trackear consumo aproximado por turn/session/project sin que el cliente IDE reporte tokens reales."),
		mcp.WithString("prompt_id",
			mcp.Description("UUID del row de captured_prompts (lo devolvio domain_prompt_capture al inicio del turn)"),
			mcp.Required(),
		),
		mcp.WithNumber("response_chars",
			mcp.Description("Total de chars que escribio el LLM en su respuesta + tool calls de este turn"),
			mcp.Required(),
		),
		mcp.WithString("model",
			mcp.Description("Modelo en uso (claude-opus-4-7, gpt-4o, etc). Opcional — si se omite mantiene el del Capture"),
		),
	)
}

func toolUsageSummary() mcp.Tool {
	return mcp.NewTool("domain_usage_summary",
		mcp.WithDescription("Agrega tokens estimados (proxy chars/4) de todos los turns de un project. Util para mostrarle al usuario cuanto esta consumiendo y detectar overengineering ('gastar mas tokens no significa ser mas productivo')."),
		mcp.WithString("project_slug",
			mcp.Description("Slug del proyecto a resumir"),
		),
	)
}

func (h *capturedPromptHandlers) handleTurnComplete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.prompts == nil {
		return mcp.NewToolResultError("captured_prompts service not configured"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["prompt_id"].(string)
	pid, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("prompt_id invalido (UUID requerido)"), nil
	}
	rc := 0
	if v, ok := args["response_chars"].(float64); ok {
		rc = int(v)
	}
	model, _ := args["model"].(string)
	orgID, _ := uuid.Parse(h.principal.OrganizationID)

	p, err := h.prompts.CompleteTurn(ctx, capturedpromptsvc.CompleteTurnInput{
		OrganizationID: orgID,
		PromptID:       pid,
		ResponseChars:  rc,
		Model:          model,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("complete turn failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"id":                   p.ID,
		"response_chars":       p.ResponseChars,
		"estimated_tokens_in":  p.EstimatedTokensIn,
		"estimated_tokens_out": p.EstimatedTokensOut,
		"turn_completed_at":    p.TurnCompletedAt,
	})
}

func (h *capturedPromptHandlers) handleUsageSummary(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.prompts == nil {
		return mcp.NewToolResultError("captured_prompts service not configured"), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	args := req.GetArguments()
	projSlug, _ := args["project_slug"].(string)

	if projSlug == "" {
		return mcp.NewToolResultError("debe pasarse project_slug"), nil
	}
	if h.projects == nil {
		return mcp.NewToolResultError("projects service not configured"), nil
	}
	proj, err := h.projects.GetBySlug(ctx, orgID, projSlug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", projSlug)), nil
	}
	u, err := h.prompts.SummarizeByProject(ctx, orgID, proj.ID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("summary failed: %v", err)), nil
	}
	return toolResultJSON(u)
}

func (h *capturedPromptHandlers) handlePromptCapturedList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if h.prompts == nil {
		return mcp.NewToolResultError("captured_prompts service not configured"), nil
	}
	orgID, err := uuid.Parse(h.principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	args := req.GetArguments()
	filter := capturedpromptsvc.ListFilter{}

	if v, ok := args["project_slug"].(string); ok && v != "" && h.projects != nil {
		if proj, perr := h.projects.GetBySlug(ctx, orgID, v); perr == nil && proj != nil {
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

	list, total, err := h.prompts.List(ctx, orgID, filter)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"prompts": list,
		"total":   total,
	})
}
