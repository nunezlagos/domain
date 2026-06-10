// Package orchestration — patrones multi-agent (issue-08.4 a issue-08.9).
//
// Cubre 6 patterns:
//   - Orquestación básica: issue-08.4 multi-agent-orch
//   - Templates: issue-08.5 agent-templates (definitions reutilizables)
//   - Supervisor: issue-08.6 multi-agent-supervisor (un agent dirige a otros)
//   - Handoff: issue-08.7 agent-handoff (agent A transfiere a B)
//   - Parallel fanout: issue-08.8 agent-parallel-fanout
//   - Hierarchical context: issue-08.9 agent-hierarchical-context (parent visión)
//
// Diseño: cada pattern es una struct + Run method que recibe contexto
// común (Conductor) y devuelve OrchestrationResult. El Conductor abstrae
// LLM calls + agent dispatch + tracking de runs anidados.
package orchestration

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Pattern identifica el tipo de orchestration.
type Pattern string

const (
	PatternSequential    Pattern = "sequential"      // issue-08.4 básico
	PatternSupervisor    Pattern = "supervisor"      // issue-08.6
	PatternHandoff       Pattern = "handoff"         // issue-08.7
	PatternParallelFanout Pattern = "parallel_fanout" // issue-08.8
)

// AgentTemplate es una definition reutilizable (issue-08.5).
type AgentTemplate struct {
	ID            uuid.UUID       `json:"id"`
	Slug          string          `json:"slug"`
	Name          string          `json:"name"`
	SystemPrompt  string          `json:"system_prompt"`
	Personality   string          `json:"personality,omitempty"`
	Capabilities  []string        `json:"capabilities,omitempty"`  // skill slugs disponibles
	Model         string          `json:"model"`
	Temperature   float32         `json:"temperature"`
	MaxTokens     int             `json:"max_tokens"`
	HandoffPolicy string          `json:"handoff_policy,omitempty"` // allow|forbid|require_supervisor_approval
	Metadata      json.RawMessage `json:"metadata,omitempty"`
}

// HandoffPolicy values.
const (
	HandoffAllow                  = "allow"
	HandoffForbid                 = "forbid"
	HandoffRequireSupervisor      = "require_supervisor_approval"
)

// Task es trabajo asignable a un agent.
type Task struct {
	ID            uuid.UUID       `json:"id"`
	Description   string          `json:"description"`
	Input         json.RawMessage `json:"input,omitempty"`
	AssignedAgent string          `json:"assigned_agent,omitempty"` // slug
	Parent        *uuid.UUID      `json:"parent_task_id,omitempty"`
	Status        string          `json:"status"` // pending|running|done|failed
	Result        json.RawMessage `json:"result,omitempty"`
	Error         string          `json:"error,omitempty"`
	StartedAt     *time.Time      `json:"started_at,omitempty"`
	CompletedAt   *time.Time      `json:"completed_at,omitempty"`
}

// OrchestrationResult agrega resultados de un patrón.
type OrchestrationResult struct {
	Pattern     Pattern    `json:"pattern"`
	Tasks       []Task     `json:"tasks"`
	FinalOutput string     `json:"final_output,omitempty"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt time.Time  `json:"completed_at"`
	Successful  bool       `json:"successful"`
	Error       string     `json:"error,omitempty"`
}

// Conductor es la interfaz que el orchestration usa para dispatcher tareas.
// Implementación real conecta a agent runner + llm provider; mock para tests.
type Conductor interface {
	// RunAgent invoca un agent con una task y devuelve su output texto + result data.
	RunAgent(ctx context.Context, agentSlug string, task Task) (string, json.RawMessage, error)

	// LoadTemplate retorna la definition reutilizable de un agent slug.
	LoadTemplate(ctx context.Context, slug string) (*AgentTemplate, error)
}

// HierarchicalContext (issue-08.9) snapshot del context jerárquico que un
// agent hijo recibe de sus parents.
type HierarchicalContext struct {
	ParentTaskID  *uuid.UUID      `json:"parent_task_id,omitempty"`
	ParentSummary string          `json:"parent_summary,omitempty"`
	SharedData    json.RawMessage `json:"shared_data,omitempty"`
	Depth         int             `json:"depth"`
}

// BuildHierarchicalContext construye el context completo para un agent
// hijo recorriendo los parents hasta root.
func BuildHierarchicalContext(tasks []Task, currentTaskID uuid.UUID) HierarchicalContext {
	taskByID := map[uuid.UUID]Task{}
	for _, t := range tasks {
		taskByID[t.ID] = t
	}
	cur, ok := taskByID[currentTaskID]
	if !ok || cur.Parent == nil {
		return HierarchicalContext{Depth: 0}
	}
	depth := 1
	parent, ok := taskByID[*cur.Parent]
	if !ok {
		return HierarchicalContext{Depth: depth, ParentTaskID: cur.Parent}
	}
	// Recorrer hasta root para profundidad
	cursor := parent
	for cursor.Parent != nil {
		next, ok := taskByID[*cursor.Parent]
		if !ok {
			break
		}
		depth++
		cursor = next
	}
	summary := parent.Description
	if len(summary) > 240 {
		summary = summary[:240] + "..."
	}
	return HierarchicalContext{
		ParentTaskID:  &parent.ID,
		ParentSummary: summary,
		Depth:         depth,
	}
}
