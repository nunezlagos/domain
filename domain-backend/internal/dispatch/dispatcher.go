// Package dispatch — issue-35.1 unified dispatcher para cron, webhook,
// MCP. Centraliza la lógica de "qué runner usar" según target_type.
// Antes había 3 switches duplicados (uno por source) que divergían
// silenciosamente: agregar un step_type nuevo y olvidarse de uno
// rompía ese source sin que el resto del sistema se enterara.
//
// Esta versión: 1 solo switch + 3 call-sites que delegan.
//
// El dispatcher NO agrega funcionalidades nuevas: es refactor. El
// comportamiento observable (qué corre, con qué timeout, qué se
// audita) es el mismo que antes — solo que ahora vive en 1 lugar.
package dispatch

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
)

// Request input normalizado al dispatcher. Los 3 call-sites (cron,
// webhook, MCP) lo construyen desde sus inputs nativos.
type Request struct {
	OrgID       uuid.UUID
	Source      string // "cron" | "webhook" | "mcp" | "manual" | otro (warning log)
	TargetType  string // "flow" | "agent" | "skill"
	TargetID    uuid.UUID
	Inputs      json.RawMessage
	TriggeredBy *uuid.UUID // user id si aplica
}

// Result output normalizado.
type Result struct {
	RunID  uuid.UUID
	Status string // "started" | "completed" | "failed"
	Output json.RawMessage
}

// Source constants para evitar typos en call-sites.
const (
	SourceCron    = "cron"
	SourceWebhook = "webhook"
	SourceMCP     = "mcp"
	SourceManual  = "manual"
)

// TargetType constants.
const (
	TargetFlow  = "flow"
	TargetAgent = "agent"
	TargetSkill = "skill"
)

// RunFunc firma de un runner (flow, agent o skill). El dispatcher
// recibe 3 funciones con esta misma firma, una por target type.
// Esta indirección permite:
//   - Inyectar stubs en tests (sin DB / LLM / etc).
//   - En producción, cada func envuelve el runner real (e.g.,
//     flowRunner.Run → func).
type RunFunc func(ctx context.Context, req Request) (Result, error)

// Dispatcher centraliza la decisión "qué runner usar" según
// target_type. Los 3 call-sites (cron, webhook, MCP) construyen un
// Request y llaman a Dispatch — la lógica vive acá, en 1 lugar.
//
// Auditoría y métricas:
//   - AuditRecorder: si no nil, registra "dispatch.started" +
//     "dispatch.completed" (entity_id = target_id).
//   - Metrics: si no nil, observa DispatchTotal
//     (labels: source, target_type, result) + DispatchDuration.
//
// SourceValidator: si retorna false para un source, el dispatcher
// loggea warning pero NO falla (permite instrumentar nuevos
// sources). Default: nil → todos válidos.
type Dispatcher struct {
	RunFlow  RunFunc
	RunAgent RunFunc
	RunSkill RunFunc

	Audit           AuditRecorder
	Metrics         MetricsRecorder
	Logger          *slog.Logger
	SourceValidator func(source string) bool
}

// AuditRecorder interface mínima. La real es audit.Recorder pero
// acá no la importamos para no acoplar dispatch → audit (que tiene
// deps de DB). El adapter en producción es interno.
type AuditRecorder interface {
	Record(ctx context.Context, e AuditEvent) error
}

// AuditEvent subset del audit.Event real. Solo lo que dispatch usa.
type AuditEvent struct {
	OrgID      uuid.UUID
	EntityType string
	EntityID   uuid.UUID
	Action     string
	Metadata   map[string]any
}

// MetricsRecorder interface mínima para no acoplar dispatch → metrics
// (que importa prometheus).
type MetricsRecorder interface {
	ObserveDispatch(source, targetType, result string, durationSeconds float64)
}
