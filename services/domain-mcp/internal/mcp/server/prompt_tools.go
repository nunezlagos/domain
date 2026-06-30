package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	prouter "nunezlagos/domain/internal/service/promptrouter"
)

type promptRouterService interface {
	RouteWithIntent(ctx context.Context, rawText string, createdBy *uuid.UUID, orgID *uuid.UUID, projectID *uuid.UUID, intentOverride *prouter.Intent) (*prouter.Response, error)
}

type promptHandlers struct {
	router    promptRouterService
	principal *apikey.Principal
}

// toolPromptRoute — domain_prompt
//
// Es el UNICO MCP tool que el agente IA necesita conocer para arrancar.
// El router clasifica intent y decide si responde directo (chat/idea) o
// arranca el wizard SDD (feature/fix/hotfix/refactor/doc/rfc).
func toolPromptRoute() mcp.Tool {
	return mcp.NewTool("domain_prompt",
		mcp.WithDescription("Entry point principal del flow Domain. Recibe un prompt crudo del usuario, lo clasifica (chat/idea/feature/fix/hotfix/refactor/doc/rfc/analysis) y devuelve: para chat/idea una respuesta directa; para el resto, arranca el wizard/orquestador y devuelve la primera pregunta. El cliente sigue con domain_hu_create_answer. CLASIFICACION HIBRIDA: como agente IA puede clasificar usted mismo el intent usando el prompt 'triage' (traelo con domain_prompt_get(slug='triage')) y pasar el resultado en el parametro 'intent' — eso SALTEA la clasificacion del servidor. Si no pasas 'intent', el servidor clasifica (LLM si hay provider, else keywords)."),
		mcp.WithString("raw_text",
			mcp.Description("Prompt crudo del usuario tal cual fue tipeado en el agente IA"),
			mcp.Required(),
		),
		mcp.WithString("created_by_user_id",
			mcp.Description("UUID del usuario que tipeo el prompt (opcional, para audit)"),
		),
		mcp.WithString("intent",
			mcp.Description("Intent ya clasificado por el cliente (opcional): chat|idea|feature|fix|hotfix|refactor|doc|rfc|analysis. Si es valido, el servidor lo usa y NO reclasifica. Usa el prompt 'triage' (domain_prompt_get) para decidirlo."),
		),
		mcp.WithString("project_id",
			mcp.Description("UUID del proyecto (de domain_session_bootstrap). OBLIGATORIO para intents SDD (feature/fix/hotfix/refactor/doc/rfc/analysis): el intake y el orquestador lo exigen y devuelven project_id required si falta. Opcional solo para chat/idea (no arrancan flujo). Resolve el project_id con domain_session_bootstrap al inicio de la sesion."),
		),
	)
}

func (h *promptHandlers) handlePromptRoute(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.router == nil {
		return mcp.NewToolResultError("prompt router not configured"), nil
	}
	rawText, err := req.RequireString("raw_text")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	var createdBy *uuid.UUID
	var orgID *uuid.UUID
	if h.principal != nil {
		if u, err := uuid.Parse(h.principal.UserID); err == nil {
			createdBy = &u
		}
		if o, err := uuid.Parse(h.principal.OrganizationID); err == nil {
			orgID = &o
		}
	}

	if s := req.GetString("created_by_user_id", ""); s != "" {
		if u, err := uuid.Parse(s); err == nil {
			createdBy = &u
		}
	}

	intentOverride := prouter.ParseIntent(req.GetString("intent", ""))

	var projectID *uuid.UUID
	if s := req.GetString("project_id", ""); s != "" {
		if p, perr := uuid.Parse(s); perr == nil {
			projectID = &p
		} else {
			return mcp.NewToolResultError("invalid project_id"), nil
		}
	}

	resp, err := h.router.RouteWithIntent(ctx, rawText, createdBy, orgID, projectID, intentOverride)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("route: %v", err)), nil
	}

	body, _ := json.MarshalIndent(resp, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(body))},
	}, nil
}

// registerPromptTools agrega tools del prompt router al listado.
func registerPromptTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := &promptHandlers{router: deps.PromptRouter, principal: deps.Principal}
	return []mcpgo.ServerTool{
		{Tool: toolPromptRoute(), Handler: wrap.Wrap("domain_prompt", h.handlePromptRoute)},
	}
}
