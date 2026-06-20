// MCP tools para intake pipeline — issue-04.8

package mcpserver

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	intakesvc "nunezlagos/domain/internal/service/intake"
)

// toolIntakeSubmit — domain_intake_submit
func toolIntakeSubmit() mcp.Tool {
	return mcp.NewTool("domain_intake_submit",
		mcp.WithDescription("Ingesta un requerimiento desde cualquier fuente en el pipeline de intake."),
		mcp.WithString("source",
			mcp.Description("Fuente: agent | email | webhook | slack | sheet | manual"),
			mcp.Required(),
		),
		mcp.WithString("raw_text",
			mcp.Description("Texto crudo del requerimiento"),
			mcp.Required(),
		),
		mcp.WithString("source_ref",
			mcp.Description("Referencia externa (ej. email ID, ticket ID)"),
		),
		mcp.WithString("submitted_by",
			mcp.Description("Identificador del remitente"),
		),
		mcp.WithObject("raw_payload",
			mcp.Description("Payload opcional adicional (JSONB)"),
		),
	)
}

func toolIntakeGet() mcp.Tool {
	return mcp.NewTool("domain_intake_get",
		mcp.WithDescription("Recupera un intake payload por ID."),
		mcp.WithString("id",
			mcp.Description("UUID del intake"),
			mcp.Required(),
		),
	)
}

func toolIntakeListPending() mcp.Tool {
	return mcp.NewTool("domain_intake_list_pending",
		mcp.WithDescription("Lista intakes pendientes de revisión (no terminales)."),
		mcp.WithNumber("limit",
			mcp.Description("Máximo resultados (default 50, max 200)"),
		),
	)
}

func toolIntakeApprove() mcp.Tool {
	return mcp.NewTool("domain_intake_approve",
		mcp.WithDescription("Aprueba un intake en pending_review. Precondición para commit."),
		mcp.WithString("id",
			mcp.Description("UUID del intake"),
			mcp.Required(),
		),
	)
}

func toolIntakeReject() mcp.Tool {
	return mcp.NewTool("domain_intake_reject",
		mcp.WithDescription("Rechaza un intake con razón."),
		mcp.WithString("id",
			mcp.Description("UUID del intake"),
			mcp.Required(),
		),
		mcp.WithString("reason",
			mcp.Description("Motivo del rechazo"),
			mcp.Required(),
		),
	)
}

// --- handlers ---

func (d *Deps) handleIntakeSubmit(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Intake == nil {
		return mcp.NewToolResultError("intake service no configurado"), nil
	}
	args := req.GetArguments()
	source, _ := args["source"].(string)
	rawText, _ := args["raw_text"].(string)
	if source == "" || rawText == "" {
		return mcp.NewToolResultError("source y raw_text son requeridos"), nil
	}
	sourceRef, _ := args["source_ref"].(string)
	submittedBy, _ := args["submitted_by"].(string)
	var rawPayload map[string]any
	if v, ok := args["raw_payload"].(map[string]any); ok {
		rawPayload = v
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	p, err := d.Intake.Submit(ctx, intakesvc.SubmitInput{
		Source:         source,
		SourceRef:      sourceRef,
		OrganizationID: &orgID,
		SubmittedBy:    submittedBy,
		RawText:        rawText,
		RawPayload:     rawPayload,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("submit: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"id":         p.ID.String(),
		"source":     p.Source,
		"status":     p.Status,
		"created_at": p.CreatedAt,
	})
}

func (d *Deps) handleIntakeGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Intake == nil {
		return mcp.NewToolResultError("intake service no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id inválido"), nil
	}
	p, err := d.Intake.Get(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get: %v", err)), nil
	}
	return toolResultJSON(p)
}

func (d *Deps) handleIntakeListPending(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Intake == nil {
		return mcp.NewToolResultError("intake service no configurado"), nil
	}
	args := req.GetArguments()
	limit := 50
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	results, err := d.Intake.ListPending(ctx, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"results": results, "count": len(results)})
}

func (d *Deps) handleIntakeApprove(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Intake == nil {
		return mcp.NewToolResultError("intake service no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id inválido"), nil
	}
	reviewerID, _ := uuid.Parse(d.Principal.UserID)
	p, err := d.Intake.Approve(ctx, id, reviewerID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("approve: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"id":     p.ID.String(),
		"status": p.Status,
	})
}

func (d *Deps) handleIntakeReject(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Intake == nil {
		return mcp.NewToolResultError("intake service no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	reason, _ := args["reason"].(string)
	if reason == "" {
		return mcp.NewToolResultError("reason es requerido"), nil
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id inválido"), nil
	}
	reviewerID, _ := uuid.Parse(d.Principal.UserID)
	p, err := d.Intake.Reject(ctx, id, reviewerID, reason)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("reject: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"id":     p.ID.String(),
		"status": p.Status,
	})
}

// registerIntakeTools agrega tools de intake al listado.
func registerIntakeTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	return []mcpgo.ServerTool{
		{Tool: toolIntakeSubmit(), Handler: wrap.Wrap("domain_intake_submit", deps.handleIntakeSubmit)},
		{Tool: toolIntakeGet(), Handler: wrap.Wrap("domain_intake_get", deps.handleIntakeGet)},
		{Tool: toolIntakeListPending(), Handler: wrap.Wrap("domain_intake_list_pending", deps.handleIntakeListPending)},
		{Tool: toolIntakeApprove(), Handler: wrap.Wrap("domain_intake_approve", deps.handleIntakeApprove)},
		{Tool: toolIntakeReject(), Handler: wrap.Wrap("domain_intake_reject", deps.handleIntakeReject)},
	}
}
