package dispatch

import (
	"encoding/json"

	"github.com/google/uuid"
)

const metaFlowContextKey = "_flow_ctx"

// FlowRunContext carries orchestration state when an agent is invoked as
// part of an SDD pipeline phase. The orchestrator injects this into
// Request.Metadata so the agent runner can provide structured context
// to the agent without the caller inheriting the full orchestrator
// conversation history.
//
// Keeping this as structured data (not prose) is intentional: it avoids
// token inflation and lets the runner decide how much to surface.
type FlowRunContext struct {
	FlowRunID         uuid.UUID      `json:"flow_run_id"`
	OrchestratorRunID uuid.UUID      `json:"orchestrator_run_id,omitempty"`
	PhaseSlug         string         `json:"phase_slug,omitempty"`
	PriorOutputs      map[string]any `json:"prior_outputs,omitempty"`
	SkillsAvailable   []string       `json:"skills_available,omitempty"`
	SkillThreshold    float64        `json:"skill_threshold,omitempty"`
	ExecMode          string         `json:"exec_mode,omitempty"`
}

// InjectIntoMetadata embeds the FlowRunContext into a metadata map under
// the reserved key. Allocates the map if nil. Returns the (possibly new) map.
func (f *FlowRunContext) InjectIntoMetadata(meta map[string]any) map[string]any {
	if meta == nil {
		meta = make(map[string]any, 1)
	}
	b, _ := json.Marshal(f)
	meta[metaFlowContextKey] = json.RawMessage(b)
	return meta
}

// ExtractFlowContext reads a FlowRunContext from a metadata map.
// Returns nil if absent or malformed — always safe to call.
func ExtractFlowContext(meta map[string]any) *FlowRunContext {
	if meta == nil {
		return nil
	}
	raw, ok := meta[metaFlowContextKey]
	if !ok {
		return nil
	}
	var b []byte
	switch v := raw.(type) {
	case []byte:
		b = v
	case json.RawMessage:
		b = []byte(v)
	case string:
		b = []byte(v)
	default:
		return nil
	}
	var f FlowRunContext
	if err := json.Unmarshal(b, &f); err != nil {
		return nil
	}
	return &f
}
