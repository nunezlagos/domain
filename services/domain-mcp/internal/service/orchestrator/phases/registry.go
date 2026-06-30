// Package phases — contrato y registry de las fases SDD del orquestador.
//
// Cada PhaseSlug (sdd-explore, sdd-spec, …) tiene un Handler que sabe:
//   - construir el prompt input para el agente IA (system + user)
//   - aplicar retry_policy específica de la fase
//   - validar la respuesta del cliente IDE (incluyendo suggested_saves
//     required=true del D5)
//
// Esta capa es el límite entre el Service que orquesta el DAG y la
// implementación concreta de cada fase. El registro permite añadir
// fases nuevas sin tocar el service principal.
package phases

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Errors propios del registry. Los errores del contrato del orquestador
// (ErrRequiredSaveMissing, etc.) viven en internal/service/orchestrator.
var (
	ErrPhaseNotRegistered = errors.New("phase slug not registered")
	ErrPhaseAlreadyExists = errors.New("phase slug already registered")
)

// PhaseSlug duplica el tipo de internal/service/orchestrator para evitar
// el ciclo de import phases → orchestrator → phases. El service convierte
// entre ambos al pasar input/output.
type PhaseSlug string

// Input es lo que el registry pasa al handler. El contexto del flow_run
// llega ya resuelto por el service (org, user, flow_run_step_id).
type Input struct {
	OrganizationID  uuid.UUID
	UserID          uuid.UUID
	FlowRunID       uuid.UUID
	FlowRunStepID   uuid.UUID
	PhaseSlug       PhaseSlug



	PriorOutputs map[PhaseSlug]map[string]any


	RawText string
}

// Output es lo que produce el handler para que el service lo persista en
// flow_run_steps.outputs JSONB.
type Output struct {



	AgentTemplateSlug string


	SystemPrompt string
	UserPrompt   string



	SuggestedSaves []SuggestedSave


	SkillThreshold float64


	RetryPolicy RetryPolicy
}

// SuggestedSave es una memoria que la fase recomienda al cliente que
// persista (vía mem_save) antes de marcar phase_result. RFC 0006 D5
// define cuáles fases tienen Required=true:
//   - sdd-design → suggested_save type=adr Required=true
//   - sdd-apply  → suggested_save type=code_reference Required=true
//   - sdd-judge  → suggested_save type=sabotage_record Required=true
type SuggestedSave struct {
	Type     string // adr | code_reference | sabotage_record | knowledge_doc | …
	Required bool

	Hint string
}

// RetryPolicy aplica cuando heartbeat-watcher (issue-08.11) detecta que
// el step quedó colgado. RFC 0006 mapea políticas a saga events:
//   - "require-cleanup" → cleanup_required
//   - "re-emit"          → reemit_eligible
//   - "" (default)       → auto_retry_eligible
type RetryPolicy string

const (
	RetryAutoEligible RetryPolicy = ""
	RetryReemit       RetryPolicy = "re-emit"
	RetryCleanup      RetryPolicy = "require-cleanup"
)

// Handler es la unidad de lógica por fase. Stateless; el service
// inyecta lo que necesite vía closures al registrar.
type Handler interface {
	Slug() PhaseSlug
	Build(ctx context.Context, in Input) (*Output, error)



	Validate(ctx context.Context, out *Output, clientResult ClientResult) error
}

// ClientResult es lo que el cliente IDE reporta vía MCP tool
// domain_orchestrate_phase_result al terminar la fase. Output contiene
// el texto final del agente; MemoryRefsSaved son los IDs de las
// observations/ADRs/sabotage_records que el cliente persistió.
type ClientResult struct {
	Output           map[string]any
	MemoryRefsSaved  []MemoryRef
	DurationMS       int64
	StartedAt        time.Time
	FinishedAt       time.Time
}

// MemoryRef apunta a una memoria persistida. Type debe coincidir con
// alguno de los SuggestedSave declarados en Output para que el
// Validate del handler la considere satisfecha.
type MemoryRef struct {
	Type string
	ID   uuid.UUID
}

// Registry mantiene el mapa slug → handler. Concurrente safe; el
// service registra al boot y consulta concurrent durante ejecución.
type Registry struct {
	mu       sync.RWMutex
	handlers map[PhaseSlug]Handler
}

// NewRegistry construye un registry vacío. Los handlers se agregan via
// Register; el orden no importa porque la secuencia del DAG vive en el
// seeder de flow:sdd-pipeline-v1 (seed-003), no acá.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[PhaseSlug]Handler)}
}

// Register agrega un handler. Devuelve ErrPhaseAlreadyExists si el slug
// ya tenía handler — evita registros duplicados accidentales en init.
func (r *Registry) Register(h Handler) error {
	if h == nil {
		return errors.New("phases: cannot register nil handler")
	}
	slug := h.Slug()
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.handlers[slug]; exists {
		return ErrPhaseAlreadyExists
	}
	r.handlers[slug] = h
	return nil
}

// MustRegister es el sugar para boot-time wiring. Panic si duplica.
func (r *Registry) MustRegister(h Handler) {
	if err := r.Register(h); err != nil {
		panic(err)
	}
}

// Lookup resuelve slug → handler. ErrPhaseNotRegistered si no está.
func (r *Registry) Lookup(slug PhaseSlug) (Handler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[slug]
	if !ok {
		return nil, ErrPhaseNotRegistered
	}
	return h, nil
}

// Slugs lista todos los handlers registrados (orden indeterminado).
// Útil para tests y para inspección runtime via debug endpoint.
func (r *Registry) Slugs() []PhaseSlug {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]PhaseSlug, 0, len(r.handlers))
	for s := range r.handlers {
		out = append(out, s)
	}
	return out
}
