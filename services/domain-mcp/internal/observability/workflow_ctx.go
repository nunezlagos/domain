// Package observability: este archivo cubre la propagacion de workflow_id
// via context.Context. workflow_id es un UUID v7 (timestamp-ordered) que
// correlaciona tool invocations + HTTP requests + fn calls + SQL queries
// en un workflow logico (sesion de agente, intake, flow run, etc).
//
// issue-53.8 workflow-correlation.
package observability

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// workflowIDKey es la clave privada en ctx para workflow_id.
type workflowIDKey struct{}

// workflowStartKey es la clave privada en ctx para unix timestamp de
// inicio del workflow (sirve para calcular duraciones agregadas).
type workflowStartKey struct{}

// workflowNameKey es la clave privada en ctx para nombre canonico.
type workflowNameKey struct{}

// WithWorkflowID retorna un ctx con workflow_id seteado.
// Si id es uuid.Nil se genera uno nuevo. Reemplaza el anterior.
func WithWorkflowID(ctx context.Context, id uuid.UUID) context.Context {
	if id == uuid.Nil {
		id = NewWorkflowID()
	}
	return context.WithValue(ctx, workflowIDKey{}, id)
}

// WithWorkflowName retorna un ctx con workflow_name seteado (auto-tag
// initiator: "issue_create", "mem_save", etc).
func WithWorkflowName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, workflowNameKey{}, name)
}

// WithWorkflowStart retorna un ctx con workflow_start_seteado.
// Si start es zero, usa time.Now(). Sirve para workflows que ya tienen
// un inicio conocido (e.g. cuando el cliente paso X-Workflow-Id al inicio).
func WithWorkflowStart(ctx context.Context, start time.Time) context.Context {
	if start.IsZero() {
		start = time.Now()
	}
	return context.WithValue(ctx, workflowStartKey{}, start)
}

// WorkflowIDFromContext devuelve el UUID v7 seteado por WithWorkflowID.
// uuid.Nil si ausente.
func WorkflowIDFromContext(ctx context.Context) uuid.UUID {
	if v, ok := ctx.Value(workflowIDKey{}).(uuid.UUID); ok {
		return v
	}
	return uuid.Nil
}

// WorkflowNameFromContext devuelve el nombre canonico. "" si ausente.
func WorkflowNameFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(workflowNameKey{}).(string); ok {
		return v
	}
	return ""
}

// WorkflowStartFromContext devuelve el start timestamp. time.Time{} si ausente.
func WorkflowStartFromContext(ctx context.Context) time.Time {
	if v, ok := ctx.Value(workflowStartKey{}).(time.Time); ok {
		return v
	}
	return time.Time{}
}

// NewWorkflowID genera un UUID v7 timestamp-ordered.
// Si la generacion falla (extremadamente raro), devuelve uuid.Nil.
// V7 embebe timestamp en los primeros bits — sortable naturalmente.
func NewWorkflowID() uuid.UUID {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil
	}
	return id
}

// EnsureWorkflowID chequea el ctx: si no tiene workflow_id, genera uno,
// lo setea, y devuelve el ctx actualizado + el id. Uso tipico en handlers:
//
//	id, ctx := observability.EnsureWorkflowID(r.Context())
//	defer observability.Tracker().Touch(ctx) // actualiza last_activity_at
func EnsureWorkflowID(ctx context.Context) (uuid.UUID, context.Context) {
	id := WorkflowIDFromContext(ctx)
	if id == uuid.Nil {
		id = NewWorkflowID()
		ctx = WithWorkflowID(ctx, id)
	}
	return id, ctx
}
