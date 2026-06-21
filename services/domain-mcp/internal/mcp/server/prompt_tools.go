// MCP tools — single-shot prompt router issue-12.7

package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	prouter "nunezlagos/domain/internal/service/promptrouter"
)

// toolPromptRoute — domain_prompt
//
// Es el ÚNICO MCP tool que el agente IA necesita conocer para arrancar.
// El router clasifica intent y decide si responde directo (chat/idea) o
// arranca el wizard SDD (feature/fix/hotfix/refactor/doc/rfc).
func toolPromptRoute() mcp.Tool {
	return mcp.NewTool("domain_prompt",
		mcp.WithDescription("Entry point principal del flow Domain. Recibe un prompt crudo del usuario, lo clasifica (chat/idea/feature/fix/hotfix/refactor/doc/rfc/analysis) y devuelve: para chat/idea una respuesta directa; para el resto, arranca el wizard/orquestador y devuelve la primera pregunta. El cliente sigue con domain_hu_create_answer. CLASIFICACIÓN HÍBRIDA: como agente IA podés clasificar vos mismo el intent usando el prompt 'triage' (traélo con domain_prompt_get(slug='triage')) y pasar el resultado en el parámetro 'intent' — eso SALTEA la clasificación del servidor. Si no pasás 'intent', el servidor clasifica (LLM si hay provider, else keywords)."),
		mcp.WithString("raw_text",
			mcp.Description("Prompt crudo del usuario tal cual fue tipeado en el agente IA"),
			mcp.Required(),
		),
		mcp.WithString("created_by_user_id",
			mcp.Description("UUID del usuario que tipeó el prompt (opcional, para audit)"),
		),
		mcp.WithString("intent",
			mcp.Description("Intent ya clasificado por el cliente (opcional): chat|idea|feature|fix|hotfix|refactor|doc|rfc|analysis. Si es válido, el servidor lo usa y NO reclasifica. Usá el prompt 'triage' (domain_prompt_get) para decidirlo."),
		),
	)
}

func (d *Deps) handlePromptRoute(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.PromptRouter == nil {
		return mcp.NewToolResultError("prompt router not configured"), nil
	}
	rawText, err := req.RequireString("raw_text")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	var createdBy *uuid.UUID
	var orgID *uuid.UUID
	if d.Principal != nil {
		if u, err := uuid.Parse(d.Principal.UserID); err == nil {
			createdBy = &u
		}
		if o, err := uuid.Parse(d.Principal.OrganizationID); err == nil {
			orgID = &o
		}
	}
	// Override desde args si el caller lo pasa explícito (tests, batch)
	if s := req.GetString("created_by_user_id", ""); s != "" {
		if u, err := uuid.Parse(s); err == nil {
			createdBy = &u
		}
	}

	intentOverride := prouter.ParseIntent(req.GetString("intent", ""))

	resp, err := d.PromptRouter.RouteWithIntent(ctx, rawText, createdBy, orgID, intentOverride)
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
	return []mcpgo.ServerTool{
		{Tool: toolPromptRoute(), Handler: wrap.Wrap("domain_prompt", deps.handlePromptRoute)},
	}
}
