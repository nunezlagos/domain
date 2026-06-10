// Package wizardplan — wizard adaptive HU-04.7 v2.
//
// Reemplaza el flow de 8 preguntas fijas (v1) por un planner que analiza
// el prompt contra 4 fuentes en paralelo y SOLO pregunta lo que no puede
// inferir.
//
// Pipeline:
//
//   prompt → Analyzer.Run (4 fuentes paralelas) → ContextEnvelope
//          → Planner.NextSlot → si todo conocido: build preview
//                             → si falta algo: LLM formula pregunta
//                                              contextualizada con envelope
//          → Answer → re-analyze slot + persiste → next slot
package wizardplan

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ContextEnvelope agrupa todo lo que el analyzer pipeline encontró sobre
// el prompt. Cada source contribuye un Finding. El planner consume el
// envelope para decidir qué slot todavía requiere input del usuario.
type ContextEnvelope struct {
	RawPrompt   string             `json:"raw_prompt"`
	GeneratedAt time.Time          `json:"generated_at"`
	Intent      *IntentFinding     `json:"intent,omitempty"`
	Memory      *MemoryFinding     `json:"memory,omitempty"`
	HUMatches   *HUDedupFinding    `json:"hu_dedup,omitempty"`
	Code        *CodeGrepFinding   `json:"code,omitempty"`
	History     *AgentHistoryFinding `json:"history,omitempty"`
	// Slots agrupa qué se sabe y qué no por slot key.
	Slots map[string]Slot `json:"slots"`
	// Errors no-críticos por source. Una source que falla no detiene el pipeline.
	SourceErrors map[string]string `json:"source_errors,omitempty"`
}

// Slot describe el estado de conocimiento de un campo requerido para
// armar la HU.
type Slot struct {
	Key        string  `json:"key"`
	Value      any     `json:"value,omitempty"`
	Status     string  `json:"status"`     // unknown | inferred | confirmed | provided
	Source     string  `json:"source"`     // intent | memory | hu_dedup | code | history | user
	Confidence float64 `json:"confidence"` // 0..1; >=0.75 → no preguntar
	Reasoning  string  `json:"reasoning,omitempty"`
}

// Status values.
const (
	SlotUnknown   = "unknown"   // necesita input del usuario
	SlotInferred  = "inferred"  // sabemos pero queremos confirmar
	SlotConfirmed = "confirmed" // el usuario confirmó la inferencia
	SlotProvided  = "provided"  // el usuario lo proveyó explícitamente
)

// IntentFinding del classifier (LLM o heurístico).
type IntentFinding struct {
	Intent     string  `json:"intent"`     // chat | idea | feature | fix | hotfix | refactor | doc | rfc
	Confidence float64 `json:"confidence"`
	Reasoning  string  `json:"reasoning"`
}

// MemoryFinding del search.Service.
type MemoryFinding struct {
	Matches []MemoryMatch `json:"matches"`
}

type MemoryMatch struct {
	EntityType string    `json:"entity_type"` // observation | knowledge | prompt
	ID         uuid.UUID `json:"id"`
	Title      string    `json:"title,omitempty"`
	Snippet    string    `json:"snippet"`
	Score      float64   `json:"score"`
}

// HUDedupFinding compara el prompt vs user_stories existentes.
type HUDedupFinding struct {
	Candidates []HUDedupCandidate `json:"candidates"`
}

type HUDedupCandidate struct {
	HUID       uuid.UUID `json:"hu_id"`
	Slug       string    `json:"slug"`
	Title      string    `json:"title"`
	Status     string    `json:"status"`
	Similarity float64   `json:"similarity"`
	Reason     string    `json:"reason"`
}

// CodeGrepFinding hits en el code base del proyecto.
type CodeGrepFinding struct {
	Hits []CodeHit `json:"hits"`
}

type CodeHit struct {
	Path     string `json:"path"`
	Line     int    `json:"line,omitempty"`
	Symbol   string `json:"symbol,omitempty"` // función / type / endpoint
	Snippet  string `json:"snippet,omitempty"`
	Category string `json:"category"` // endpoint | service | type | handler | other
}

// AgentHistoryFinding agent_runs recientes del usuario sobre topics
// relacionados.
type AgentHistoryFinding struct {
	RelatedRuns []RelatedRun `json:"related_runs"`
}

type RelatedRun struct {
	AgentRunID uuid.UUID `json:"agent_run_id"`
	AgentSlug  string    `json:"agent_slug,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	Summary    string    `json:"summary,omitempty"`
}

// Required slot keys universales para construir una HU.
// El planner los itera; cada uno tiene un Inferrer que lo intenta poblar.
const (
	SlotIntent          = "intent"
	SlotAudience        = "audience"
	SlotREQParent       = "req_parent"
	SlotComponent       = "affected_component"
	SlotSeverity        = "severity"
	SlotExpected        = "expected_behavior"
	SlotActual          = "actual_behavior"
	SlotGoal            = "goal"
	SlotSummary         = "summary"
	SlotSlug            = "slug"
)

// RequiredSlotsForIntent devuelve los slots que se necesitan para armar la
// HU según el intent detectado. Si intent es chat/idea NO entra al wizard.
func RequiredSlotsForIntent(intent string) []string {
	switch intent {
	case "fix", "hotfix":
		return []string{
			SlotIntent, SlotComponent, SlotSeverity, SlotExpected, SlotActual,
			SlotREQParent, SlotSlug, SlotSummary,
		}
	case "feature":
		return []string{
			SlotIntent, SlotAudience, SlotREQParent, SlotGoal, SlotSummary, SlotSlug,
		}
	case "refactor":
		return []string{
			SlotIntent, SlotComponent, SlotREQParent, SlotGoal, SlotSummary, SlotSlug,
		}
	case "doc":
		return []string{
			SlotIntent, SlotREQParent, SlotGoal, SlotSummary, SlotSlug,
		}
	case "rfc":
		return []string{
			SlotIntent, SlotGoal, SlotSummary, SlotSlug,
		}
	}
	return []string{}
}

// NewEnvelope inicializa un envelope con todos los required slots en
// status=unknown para un intent dado.
func NewEnvelope(rawPrompt, intent string) *ContextEnvelope {
	e := &ContextEnvelope{
		RawPrompt:   rawPrompt,
		GeneratedAt: time.Now(),
		Slots:       map[string]Slot{},
	}
	for _, k := range RequiredSlotsForIntent(intent) {
		e.Slots[k] = Slot{Key: k, Status: SlotUnknown}
	}
	return e
}

// MarshalToJSON helper. Errors se ignoran (best-effort persistence).
func (e *ContextEnvelope) MarshalToJSON() json.RawMessage {
	b, _ := json.Marshal(e)
	return b
}

// UnmarshalFromJSON helper.
func UnmarshalFromJSON(data []byte) (*ContextEnvelope, error) {
	var e ContextEnvelope
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, err
	}
	if e.Slots == nil {
		e.Slots = map[string]Slot{}
	}
	return &e, nil
}

// Touch marca un slot con un valor + source + confidence.
func (e *ContextEnvelope) Touch(key string, value any, source string, conf float64, reason string) {
	if e.Slots == nil {
		e.Slots = map[string]Slot{}
	}
	status := SlotInferred
	if source == "user" {
		status = SlotProvided
	}
	e.Slots[key] = Slot{
		Key: key, Value: value, Status: status,
		Source: source, Confidence: conf, Reasoning: reason,
	}
}

// PendingSlots devuelve los slot keys que todavía no se conocen con
// suficiente confianza (>= threshold). Default threshold = 0.75.
func (e *ContextEnvelope) PendingSlots(threshold float64) []string {
	if threshold <= 0 {
		threshold = 0.75
	}
	var pending []string
	for key, s := range e.Slots {
		switch s.Status {
		case SlotProvided, SlotConfirmed:
			continue
		case SlotInferred:
			if s.Confidence >= threshold {
				continue
			}
		}
		pending = append(pending, key)
	}
	return pending
}

// Source ejecuta una fuente de análisis y modifica el envelope in-place.
type Source interface {
	Name() string
	Run(ctx context.Context, env *ContextEnvelope) error
}
