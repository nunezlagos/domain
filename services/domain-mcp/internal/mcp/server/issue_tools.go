

package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	issuesvc "nunezlagos/domain/internal/service/issue"
	husvc "nunezlagos/domain/internal/service/issuebuilder"
)

type huService interface {
	Start(ctx context.Context, mode, initialIdea string, createdBy *uuid.UUID, projectID *uuid.UUID) (*husvc.Draft, *husvc.Question, error)
	Answer(ctx context.Context, draftID uuid.UUID, rawAnswer any) (*husvc.Draft, *husvc.Question, error)
	BuildPreview(ctx context.Context, draftID uuid.UUID) (*husvc.Preview, error)
	Commit(ctx context.Context, draftID uuid.UUID) (*husvc.Draft, error)
	Abandon(ctx context.Context, draftID uuid.UUID, reason string) error
	List(ctx context.Context, status string) ([]husvc.Draft, error)
}

type issueCRUD interface {
	GetByID(ctx context.Context, id uuid.UUID) (*issuesvc.Issue, error)
	Update(ctx context.Context, slug string, title *string, description *string, status *string, priority *string) (*issuesvc.Issue, error)
}

type issueHandlers struct {
	hu        huService
	issue     issueCRUD
	principal *apikey.Principal
}

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
		mcp.WithString("project_id",
			mcp.Description("UUID del proyecto (de domain_session_bootstrap). OBLIGATORIO: scopea el draft al proyecto (issue_drafts.project_id es NOT NULL)."),
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

func (h *issueHandlers) handleHUCreateStart(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil || h.hu == nil {
		return mcp.NewToolResultError("issuebuilder service no configurado"), nil
	}
	args := req.GetArguments()
	mode, _ := args["mode"].(string)
	idea, _ := args["initial_idea"].(string)
	if mode == "" || idea == "" {
		return mcp.NewToolResultError("mode e initial_idea son requeridos"), nil
	}
	userID, _ := uuid.Parse(h.principal.UserID)

	pidStr := req.GetString("project_id", "")
	if pidStr == "" {
		return mcp.NewToolResultError("project_id es requerido (de domain_session_bootstrap)"), nil
	}
	p, perr := uuid.Parse(pidStr)
	if perr != nil {
		return mcp.NewToolResultError("invalid project_id"), nil
	}
	projectID := &p
	draft, q, err := h.hu.Start(ctx, mode, idea, &userID, projectID)
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

func (h *issueHandlers) handleHUCreateAnswer(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil || h.hu == nil {
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

	var rawAnswer any = answer
	var parsed any
	if json.Unmarshal([]byte(answer), &parsed) == nil {
		rawAnswer = parsed
	}
	draft, q, err := h.hu.Answer(ctx, id, rawAnswer)
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

func (h *issueHandlers) handleHUCreatePreview(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil || h.hu == nil {
		return mcp.NewToolResultError("issuebuilder service no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["draft_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("draft_id invalido"), nil
	}
	preview, err := h.hu.BuildPreview(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("preview: %v", err)), nil
	}
	return toolResultJSON(preview)
}

func (h *issueHandlers) handleHUCreateCommit(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil || h.hu == nil {
		return mcp.NewToolResultError("issuebuilder service no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["draft_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("draft_id invalido"), nil
	}
	draft, err := h.hu.Commit(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("commit: %v", err)), nil
	}
	out := map[string]any{
		"draft_id":     draft.ID.String(),
		"status":       draft.Status,
		"committed_at": draft.CommittedAt,
		"target_path":  draft.TargetPath,
	}

	if draft.IssueID != nil {
		out["issue_id"] = draft.IssueID.String()
		out["issue_slug"] = draft.IssueSlug
	}
	return toolResultJSON(out)
}

func (h *issueHandlers) handleHUCreateAbandon(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil || h.hu == nil {
		return mcp.NewToolResultError("issuebuilder service no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["draft_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("draft_id invalido"), nil
	}
	reason, _ := args["reason"].(string)
	if err := h.hu.Abandon(ctx, id, reason); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("abandon: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"draft_id": idStr,
		"status":   husvc.StatusAbandoned,
	})
}

func (h *issueHandlers) handleHUDraftsList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil || h.hu == nil {
		return mcp.NewToolResultError("issuebuilder service no configurado"), nil
	}
	args := req.GetArguments()
	status, _ := args["status"].(string)
	drafts, err := h.hu.List(ctx, status)
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

func (h *issueHandlers) handleIssueSetStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.principal == nil || h.issue == nil {
		return mcp.NewToolResultError("issue service no configurado"), nil
	}
	issueIDStr := req.GetString("issue_id", "")
	statusVal := req.GetString("status", "")
	if issueIDStr == "" || statusVal == "" {
		return mcp.NewToolResultError("issue_id y status son requeridos"), nil
	}
	id, err := uuid.Parse(issueIDStr)
	if err != nil {
		return mcp.NewToolResultError("issue_id invalido"), nil
	}
	existing, err := h.issue.GetByID(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("issue no encontrada: %v", err)), nil
	}
	updated, err := h.issue.Update(ctx, existing.Slug, nil, nil, &statusVal, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("update status: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"issue_id":   updated.ID.String(),
		"slug":       updated.Slug,
		"status":     updated.Status,
		"updated_at": updated.UpdatedAt,
	})
}

// registerHUTools registra los tools del wizard de issues (HU → issue
// rename del REQ-56). Se registran tanto los NUEVOS nombres
// domain_issue_* como los LEGACY domain_hu_* para no romper clientes
// que aun tipean los nombres viejos. Los handlers son los mismos.
func registerHUTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := &issueHandlers{hu: deps.Hubuilder, issue: deps.IssueSvc, principal: deps.Principal}
	return []mcpgo.ServerTool{

		{Tool: toolIssueCreateStart(), Handler: wrap.Wrap("domain_issue_create_start", h.handleHUCreateStart)},
		{Tool: toolIssueCreateAnswer(), Handler: wrap.Wrap("domain_issue_create_answer", h.handleHUCreateAnswer)},
		{Tool: toolIssueCreatePreview(), Handler: wrap.Wrap("domain_issue_create_preview", h.handleHUCreatePreview)},
		{Tool: toolIssueCreateCommit(), Handler: wrap.Wrap("domain_issue_create_commit", h.handleHUCreateCommit)},
		{Tool: toolIssueCreateAbandon(), Handler: wrap.Wrap("domain_issue_create_abandon", h.handleHUCreateAbandon)},
		{Tool: toolIssueDraftsList(), Handler: wrap.Wrap("domain_issue_drafts_list", h.handleHUDraftsList)},
		{Tool: toolIssueSetStatus(), Handler: wrap.Wrap("domain_issue_set_status", h.handleIssueSetStatus)},

		{Tool: toolHUCreateStart(), Handler: wrap.Wrap("domain_hu_create_start", h.handleHUCreateStart)},
		{Tool: toolHUCreateAnswer(), Handler: wrap.Wrap("domain_hu_create_answer", h.handleHUCreateAnswer)},
		{Tool: toolHUCreatePreview(), Handler: wrap.Wrap("domain_hu_create_preview", h.handleHUCreatePreview)},
		{Tool: toolHUCreateCommit(), Handler: wrap.Wrap("domain_hu_create_commit", h.handleHUCreateCommit)},
		{Tool: toolHUCreateAbandon(), Handler: wrap.Wrap("domain_hu_create_abandon", h.handleHUCreateAbandon)},
		{Tool: toolHUDraftsList(), Handler: wrap.Wrap("domain_hu_drafts_list", h.handleHUDraftsList)},
	}
}

func toolIssueCreateStart() mcp.Tool {
	return mcp.NewTool("domain_issue_create_start",
		mcp.WithDescription("Inicia un wizard de creacion de issue (workflow SDD con Gherkin scenarios). Crea un borrador y devuelve la primera pregunta. NOTA: para tickets operativos (bug/task/feature basicos sin Gherkin) usar domain_ticket_create."),
		mcp.WithString("mode", mcp.Description("Modo del wizard: feature"), mcp.Required()),
		mcp.WithString("initial_idea", mcp.Description("Idea inicial del issue"), mcp.Required()),
		mcp.WithString("project_id", mcp.Description("UUID del proyecto activo (de domain_session_bootstrap)"), mcp.Required()),
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

func toolIssueSetStatus() mcp.Tool {
	return mcp.NewTool("domain_issue_set_status",
		mcp.WithDescription("Actualiza el status de una issue. Usado por sdd-archive para marcar la issue como implemented al cerrar el ciclo SDD. Valores validos: proposed | active | implemented | archived."),
		mcp.WithString("issue_id", mcp.Description("UUID de la issue a actualizar"), mcp.Required()),
		mcp.WithString("status", mcp.Description("Nuevo status: proposed | active | implemented | archived"), mcp.Required()),
	)
}
