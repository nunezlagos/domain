// MCP tools para HU builder — issue-04.7

package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	husvc "nunezlagos/domain/internal/service/issuebuilder"
)

// toolHUCreateStart — domain_hu_create_start
func toolHUCreateStart() mcp.Tool {
	return mcp.NewTool("domain_hu_create_start",
		mcp.WithDescription("Inicia un nuevo wizard de creacion de HU. Crea un borrador y devuelve la primera pregunta."),
		mcp.WithString("mode",
			mcp.Description("Modo del wizard: feature"),
			mcp.Required(),
		),
		mcp.WithString("initial_idea",
			mcp.Description("Idea inicial de la HU"),
			mcp.Required(),
		),
	)
}

func toolHUCreateAnswer() mcp.Tool {
	return mcp.NewTool("domain_hu_create_answer",
		mcp.WithDescription("Responde la pregunta actual del wizard y avanza al siguiente paso."),
		mcp.WithString("draft_id",
			mcp.Description("UUID del draft"),
			mcp.Required(),
		),
		mcp.WithString("answer",
			mcp.Description("Respuesta a la pregunta actual"),
			mcp.Required(),
		),
	)
}

func toolHUCreatePreview() mcp.Tool {
	return mcp.NewTool("domain_hu_create_preview",
		mcp.WithDescription("Genera preview de los archivos SDD a partir de las respuestas completadas."),
		mcp.WithString("draft_id",
			mcp.Description("UUID del draft (debe estar en status finished)"),
			mcp.Required(),
		),
	)
}

func toolHUCreateCommit() mcp.Tool {
	return mcp.NewTool("domain_hu_create_commit",
		mcp.WithDescription("Confirma el draft como committed. Bloquea respuestas posteriores."),
		mcp.WithString("draft_id",
			mcp.Description("UUID del draft"),
			mcp.Required(),
		),
	)
}

func toolHUCreateAbandon() mcp.Tool {
	return mcp.NewTool("domain_hu_create_abandon",
		mcp.WithDescription("Abandona un draft en progreso."),
		mcp.WithString("draft_id",
			mcp.Description("UUID del draft"),
			mcp.Required(),
		),
		mcp.WithString("reason",
			mcp.Description("Motivo del abandono"),
		),
	)
}

func toolHUDraftsList() mcp.Tool {
	return mcp.NewTool("domain_hu_drafts_list",
		mcp.WithDescription("Lista drafts de HU, filtrados por status opcional."),
		mcp.WithString("status",
			mcp.Description("Filtrar por status: in_progress | finished | committed | abandoned (opcional)"),
		),
	)
}

// --- handlers ---

func (d *Deps) handleHUCreateStart(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Hubuilder == nil {
		return mcp.NewToolResultError("issuebuilder service no configurado"), nil
	}
	args := req.GetArguments()
	mode, _ := args["mode"].(string)
	idea, _ := args["initial_idea"].(string)
	if mode == "" || idea == "" {
		return mcp.NewToolResultError("mode e initial_idea son requeridos"), nil
	}
	userID, _ := uuid.Parse(d.Principal.UserID)
	draft, q, err := d.Hubuilder.Start(ctx, mode, idea, &userID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("start: %v", err)), nil
	}
	out := map[string]any{
		"draft_id": draft.ID.String(),
		"mode":     draft.Mode,
		"status":   draft.Status,
	}
	if q != nil {
		out["question"] = q
	}
	return toolResultJSON(out)
}

func (d *Deps) handleHUCreateAnswer(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Hubuilder == nil {
		return mcp.NewToolResultError("issuebuilder service no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["draft_id"].(string)
	answer, _ := args["answer"].(string)
	if idStr == "" || answer == "" {
		return mcp.NewToolResultError("draft_id y answer son requeridos"), nil
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("draft_id invalido"), nil
	}
	// Intentar parsear answer como JSON primero; si falla, usar string literal
	var rawAnswer any = answer
	var parsed any
	if json.Unmarshal([]byte(answer), &parsed) == nil {
		rawAnswer = parsed
	}
	draft, q, err := d.Hubuilder.Answer(ctx, id, rawAnswer)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("answer: %v", err)), nil
	}
	out := map[string]any{
		"draft_id":     draft.ID.String(),
		"current_step": draft.CurrentStep,
		"total_steps":  draft.TotalSteps,
		"status":       draft.Status,
	}
	if q != nil {
		out["question"] = q
	}
	if draft.Status == husvc.StatusFinished {
		out["message"] = "All questions answered. Use domain_hu_create_preview to generate files."
	}
	return toolResultJSON(out)
}

func (d *Deps) handleHUCreatePreview(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Hubuilder == nil {
		return mcp.NewToolResultError("issuebuilder service no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["draft_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("draft_id invalido"), nil
	}
	preview, err := d.Hubuilder.BuildPreview(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("preview: %v", err)), nil
	}
	return toolResultJSON(preview)
}

func (d *Deps) handleHUCreateCommit(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Hubuilder == nil {
		return mcp.NewToolResultError("issuebuilder service no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["draft_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("draft_id invalido"), nil
	}
	draft, err := d.Hubuilder.Commit(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("commit: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"draft_id":     draft.ID.String(),
		"status":       draft.Status,
		"committed_at": draft.CommittedAt,
		"target_path":  draft.TargetPath,
	})
}

func (d *Deps) handleHUCreateAbandon(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Hubuilder == nil {
		return mcp.NewToolResultError("issuebuilder service no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["draft_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("draft_id invalido"), nil
	}
	reason, _ := args["reason"].(string)
	if err := d.Hubuilder.Abandon(ctx, id, reason); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("abandon: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"draft_id": idStr,
		"status":   husvc.StatusAbandoned,
	})
}

func (d *Deps) handleHUDraftsList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Hubuilder == nil {
		return mcp.NewToolResultError("issuebuilder service no configurado"), nil
	}
	args := req.GetArguments()
	status, _ := args["status"].(string)
	drafts, err := d.Hubuilder.List(ctx, status)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list: %v", err)), nil
	}
	out := make([]map[string]any, 0, len(drafts))
	for _, dr := range drafts {
		out = append(out, map[string]any{
			"id":           dr.ID.String(),
			"mode":         dr.Mode,
			"initial_idea": dr.InitialIdea,
			"current_step": dr.CurrentStep,
			"total_steps":  dr.TotalSteps,
			"status":       dr.Status,
			"created_at":   dr.CreatedAt,
		})
	}
	return toolResultJSON(map[string]any{"results": out, "count": len(out)})
}

// registerHUTools registra los tools del wizard de issues (HU → issue
// rename del REQ-56). Se registran tanto los NUEVOS nombres
// domain_issue_* como los LEGACY domain_hu_* para no romper clientes
// que aun tipean los nombres viejos. Los handlers son los mismos.
func registerHUTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	return []mcpgo.ServerTool{
		// Nuevos nombres (issue_create_*)
		{Tool: toolIssueCreateStart(), Handler: wrap.Wrap("domain_issue_create_start", deps.handleHUCreateStart)},
		{Tool: toolIssueCreateAnswer(), Handler: wrap.Wrap("domain_issue_create_answer", deps.handleHUCreateAnswer)},
		{Tool: toolIssueCreatePreview(), Handler: wrap.Wrap("domain_issue_create_preview", deps.handleHUCreatePreview)},
		{Tool: toolIssueCreateCommit(), Handler: wrap.Wrap("domain_issue_create_commit", deps.handleHUCreateCommit)},
		{Tool: toolIssueCreateAbandon(), Handler: wrap.Wrap("domain_issue_create_abandon", deps.handleHUCreateAbandon)},
		{Tool: toolIssueDraftsList(), Handler: wrap.Wrap("domain_issue_drafts_list", deps.handleHUDraftsList)},
		// Aliases legacy domain_hu_* (deprecados pero funcionando)
		{Tool: toolHUCreateStart(), Handler: wrap.Wrap("domain_hu_create_start", deps.handleHUCreateStart)},
		{Tool: toolHUCreateAnswer(), Handler: wrap.Wrap("domain_hu_create_answer", deps.handleHUCreateAnswer)},
		{Tool: toolHUCreatePreview(), Handler: wrap.Wrap("domain_hu_create_preview", deps.handleHUCreatePreview)},
		{Tool: toolHUCreateCommit(), Handler: wrap.Wrap("domain_hu_create_commit", deps.handleHUCreateCommit)},
		{Tool: toolHUCreateAbandon(), Handler: wrap.Wrap("domain_hu_create_abandon", deps.handleHUCreateAbandon)},
		{Tool: toolHUDraftsList(), Handler: wrap.Wrap("domain_hu_drafts_list", deps.handleHUDraftsList)},
	}
}

// --- Nuevos tool builders con nombres domain_issue_* (REQ-56).
// Los descriptions hablan de "issue" en vez de "HU".

func toolIssueCreateStart() mcp.Tool {
	return mcp.NewTool("domain_issue_create_start",
		mcp.WithDescription("Inicia un wizard de creacion de issue (workflow SDD con Gherkin scenarios). Crea un borrador y devuelve la primera pregunta. NOTA: para tickets operativos (bug/task/feature basicos sin Gherkin) usar domain_ticket_create."),
		mcp.WithString("mode", mcp.Description("Modo del wizard: feature"), mcp.Required()),
		mcp.WithString("initial_idea", mcp.Description("Idea inicial del issue"), mcp.Required()),
	)
}

func toolIssueCreateAnswer() mcp.Tool {
	return mcp.NewTool("domain_issue_create_answer",
		mcp.WithDescription("Responde la pregunta actual del wizard de issue y avanza al siguiente paso."),
		mcp.WithString("draft_id", mcp.Description("UUID del draft"), mcp.Required()),
		mcp.WithString("answer", mcp.Description("Respuesta a la pregunta actual"), mcp.Required()),
	)
}

func toolIssueCreatePreview() mcp.Tool {
	return mcp.NewTool("domain_issue_create_preview",
		mcp.WithDescription("Genera preview de los archivos SDD a partir de las respuestas completadas."),
		mcp.WithString("draft_id", mcp.Description("UUID del draft (debe estar en status finished)"), mcp.Required()),
	)
}

func toolIssueCreateCommit() mcp.Tool {
	return mcp.NewTool("domain_issue_create_commit",
		mcp.WithDescription("Confirma el draft de issue como committed. Bloquea respuestas posteriores."),
		mcp.WithString("draft_id", mcp.Description("UUID del draft"), mcp.Required()),
	)
}

func toolIssueCreateAbandon() mcp.Tool {
	return mcp.NewTool("domain_issue_create_abandon",
		mcp.WithDescription("Abandona un draft de issue en progreso."),
		mcp.WithString("draft_id", mcp.Description("UUID del draft"), mcp.Required()),
		mcp.WithString("reason", mcp.Description("Motivo del abandono")),
	)
}

func toolIssueDraftsList() mcp.Tool {
	return mcp.NewTool("domain_issue_drafts_list",
		mcp.WithDescription("Lista drafts de issue (workflow SDD), filtrables por status."),
		mcp.WithString("status", mcp.Description("Filtrar: in_progress | finished | committed | abandoned")),
	)
}
