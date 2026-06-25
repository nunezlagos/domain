// Package steptypes implements 10 standalone step runners for the flow engine.
// Each runner implements StepRunner and is registered in the Registry.
//
// Step types:
//   - skill_call   : invokes a skill by slug
//   - llm_call     : resolves prompt template, calls LLM provider
//   - code_exec    : executes sandboxed script (stub until REQ-11)
//   - conditional  : evaluates expression, branches if/else
//   - parallel     : launches N steps concurrently via errgroup
//   - wait         : waits by duration or condition polling
//   - human_input  : creates human approval task
//   - agent_run    : delegates to agent system
//   - sub_flow     : launches subflow, waits for result
//   - transform    : applies JSONPath/jq expression
package steptypes

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// StepRunner executes a single step and returns a result or error.
type StepRunner interface {
	Run(ctx context.Context, input RunInput) (any, error)
}

// RunInput carries all data and dependencies a runner needs.
type RunInput struct {
	Config      map[string]any
	Inputs      map[string]any
	StepOutputs map[string]any // step_id → result of previous steps
	OrgID       uuid.UUID
	UserID      *uuid.UUID
	DefaultTimeout time.Duration


	SkillCaller    SkillCaller
	LLMProvider    LLMProvider
	AgentRunner     AgentRunner
	SubFlowLauncher SubFlowLauncher
	TaskCreator     TaskCreator



	Heartbeat func(ctx context.Context, progress float64, message string) error
}

// SkillCaller invokes a skill by slug with args.
type SkillCaller interface {
	Call(ctx context.Context, orgID uuid.UUID, slug string, args map[string]any) (any, error)
}

// LLMProvider sends a prompt to an LLM and returns the response.
type LLMProvider interface {
	Complete(ctx context.Context, model string, prompt string, opts map[string]any) (string, error)
}

// AgentRunner delegates execution to the agent system.
type AgentRunner interface {
	Run(ctx context.Context, orgID uuid.UUID, agentSlug, input string, userID *uuid.UUID) (any, error)
}

// SubFlowLauncher launches a sub-flow and returns its result.
type SubFlowLauncher interface {
	Run(ctx context.Context, orgID uuid.UUID, flowSlug string, inputs map[string]any, userID *uuid.UUID) (any, error)
}

// TaskCreator creates a human approval task.
type TaskCreator interface {
	CreateTask(ctx context.Context, orgID uuid.UUID, question string, timeout time.Duration, params map[string]any) (string, error)
}

// Registry maps step type names to their runners.
type Registry struct {
	runners map[string]StepRunner
}

// NewRegistry creates a registry with all built-in runners registered.
func NewRegistry() *Registry {
	r := &Registry{runners: make(map[string]StepRunner, 10)}
	r.Register("skill_call", &SkillCallRunner{})
	r.Register("llm_call", &LLMCallRunner{})
	r.Register("code_exec", &CodeExecRunner{})
	r.Register("conditional", &ConditionalRunner{})
	r.Register("parallel", &ParallelRunner{})
	r.Register("wait", &WaitRunner{})
	r.Register("human_input", &HumanInputRunner{})
	r.Register("domain_agent_run", &AgentRunRunner{})
	r.Register("sub_flow", &SubFlowRunner{})
	r.Register("transform", &TransformRunner{})
	return r
}

// Register adds a runner for the given type name.
// Panics if the type is already registered.
func (r *Registry) Register(typeName string, runner StepRunner) {
	if _, ok := r.runners[typeName]; ok {
		panic(fmt.Sprintf("step type %q already registered", typeName))
	}
	r.runners[typeName] = runner
}

// Get returns the runner for the given type name, or nil if not found.
func (r *Registry) Get(typeName string) StepRunner {
	return r.runners[typeName]
}

// Has reports whether a runner is registered for the given type.
func (r *Registry) Has(typeName string) bool {
	_, ok := r.runners[typeName]
	return ok
}

// Types returns all registered type names.
func (r *Registry) Types() []string {
	out := make([]string, 0, len(r.runners))
	for k := range r.runners {
		out = append(out, k)
	}
	return out
}

// configString extracts a string value from config.
func configString(cfg map[string]any, key string) string {
	v, _ := cfg[key].(string)
	return v
}

// configInt extracts an int value from config.
func configInt(cfg map[string]any, key string) int {
	v, ok := cfg[key]
	if !ok {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case int64:
		return int(val)
	}
	return 0
}

// configMap extracts a map value from config.
func configMap(cfg map[string]any, key string) map[string]any {
	v, _ := cfg[key].(map[string]any)
	return v
}

// configSlice extracts a slice value from config.
func configSlice(cfg map[string]any, key string) []any {
	v, _ := cfg[key].([]any)
	return v
}
