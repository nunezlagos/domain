// Package observability: este archivo centraliza el logging de UNA tool
// invocation — encolar la fila de mcp_tool_invocations, tocar el workflow
// si hay uno, y emitir la linea slog "tool invocation".
//
// Vivia duplicado inline en los dos wireups (cmd/domain y cmd/domain-mcp),
// donde el invariante de workflow_id no se podia testear sin levantar un
// server entero. DOMAINSERV-212.
package observability

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// ToolCall son las metricas de UNA tool invocation medidas por el server MCP.
type ToolCall struct {
	Tool         string
	Status       string
	ErrorCode    string
	ErrorMessage string
	DurationMS   int64
}

// LogToolInvocation encola la invocacion, toca el workflow si el ctx trae uno,
// y emite la linea "tool invocation". logger/inv/tracker admiten nil.
//
// Contrato de workflow_id: sin workflow en el ctx, el campo NO se emite en el
// log ni se persiste. Emitir el uuid en ceros ensuciaba tanto los logs como la
// columna workflow_id de mcp_tool_invocations (DOMAINSERV-212).
func LogToolInvocation(ctx context.Context, logger *slog.Logger, inv *InvocationLogger, tracker *Tracker, c ToolCall) {
	if logger == nil {
		logger = slog.Default()
	}
	wfID := WorkflowIDFromContext(ctx)
	if inv != nil {
		inv.Log(Invocation{
			ToolName:     c.Tool,
			Status:       c.Status,
			DurationMS:   int(c.DurationMS),
			ErrorCode:    c.ErrorCode,
			ErrorMessage: c.ErrorMessage,
			WorkflowID:   workflowIDForRow(wfID),
		})
	}
	// Touch sigue gateado por workflow presente: generar uno por invocacion
	// era la fabrica de filas basura que corto DOMAINSERV-189
	if tracker != nil && wfID != uuid.Nil {
		tracker.Touch(ctx, WorkflowRow{
			ID:              wfID,
			Name:            WorkflowNameFromContext(ctx),
			Status:          WorkflowRunning,
			LastActivityAt:  time.Now(),
			TotalToolCalls:  1,
			TotalErrors:     boolToInt(c.Status == "error"),
			TotalDurationMS: c.DurationMS,
		})
	}
	attrs := []any{
		slog.String("tool", c.Tool),
		slog.String("status", c.Status),
		slog.String("error_code", c.ErrorCode),
		slog.String("error_message", c.ErrorMessage),
		slog.Int64("duration_ms", c.DurationMS),
	}
	if wfID != uuid.Nil {
		attrs = append(attrs, slog.String("workflow_id", wfID.String()))
	}
	logger.Info("tool invocation", attrs...)
}

// workflowIDForRow devuelve "" cuando no hay workflow, para que el NULLIF del
// INSERT deje workflow_id en NULL en vez del uuid en ceros.
func workflowIDForRow(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}

func boolToInt(cond bool) int {
	if cond {
		return 1
	}
	return 0
}
