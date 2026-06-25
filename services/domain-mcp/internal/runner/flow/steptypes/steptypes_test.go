package steptypes

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testOrgID  = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	testUserID = uuid.MustParse("00000000-0000-0000-0000-000000000002")
)



type mockSkillCaller struct {
	mu     sync.Mutex
	calls  []skillCallCall
	result any
	err    error
}

type skillCallCall struct {
	orgID uuid.UUID
	slug  string
	args  map[string]any
}

func (m *mockSkillCaller) Call(ctx context.Context, orgID uuid.UUID, slug string, args map[string]any) (any, error) {
	m.mu.Lock()
	m.calls = append(m.calls, skillCallCall{orgID: orgID, slug: slug, args: args})
	m.mu.Unlock()
	return m.result, m.err
}

type mockLLMProvider struct {
	mu      sync.Mutex
	calls   []llmCallCall
	result  string
	err     error
}

type llmCallCall struct {
	model  string
	prompt string
	opts   map[string]any
}

func (m *mockLLMProvider) Complete(ctx context.Context, model, prompt string, opts map[string]any) (string, error) {
	m.mu.Lock()
	m.calls = append(m.calls, llmCallCall{model: model, prompt: prompt, opts: opts})
	m.mu.Unlock()
	return m.result, m.err
}

type mockAgentRunner struct {
	mu     sync.Mutex
	calls  []agentRunCall
	result any
	err    error
}

type agentRunCall struct {
	orgID     uuid.UUID
	agentSlug string
	input     string
	userID    *uuid.UUID
}

func (m *mockAgentRunner) Run(ctx context.Context, orgID uuid.UUID, agentSlug, input string, userID *uuid.UUID) (any, error) {
	m.mu.Lock()
	m.calls = append(m.calls, agentRunCall{orgID: orgID, agentSlug: agentSlug, input: input, userID: userID})
	m.mu.Unlock()
	return m.result, m.err
}

type mockSubFlowRunner struct {
	mu     sync.Mutex
	calls  []subFlowCall
	result any
	err    error
}

type subFlowCall struct {
	orgID    uuid.UUID
	flowSlug string
	inputs   map[string]any
	userID   *uuid.UUID
}

func (m *mockSubFlowRunner) Run(ctx context.Context, orgID uuid.UUID, flowSlug string, inputs map[string]any, userID *uuid.UUID) (any, error) {
	m.mu.Lock()
	m.calls = append(m.calls, subFlowCall{orgID: orgID, flowSlug: flowSlug, inputs: inputs, userID: userID})
	m.mu.Unlock()
	return m.result, m.err
}

type mockTaskCreator struct {
	mu      sync.Mutex
	calls   []taskCreateCall
	taskID  string
	err     error
}

type taskCreateCall struct {
	orgID    uuid.UUID
	question string
	timeout  time.Duration
	params   map[string]any
}

func (m *mockTaskCreator) CreateTask(ctx context.Context, orgID uuid.UUID, question string, timeout time.Duration, params map[string]any) (string, error) {
	m.mu.Lock()
	m.calls = append(m.calls, taskCreateCall{orgID: orgID, question: question, timeout: timeout, params: params})
	m.mu.Unlock()
	return m.taskID, m.err
}

func defaultInput() RunInput {
	return RunInput{
		Config:      map[string]any{},
		Inputs:      map[string]any{},
		StepOutputs: map[string]any{},
		OrgID:       testOrgID,
		UserID:      &testUserID,
	}
}



func TestRegistry(t *testing.T) {
	r := NewRegistry()
	assert.True(t, r.Has("skill_call"))
	assert.True(t, r.Has("llm_call"))
	assert.True(t, r.Has("code_exec"))
	assert.True(t, r.Has("conditional"))
	assert.True(t, r.Has("parallel"))
	assert.True(t, r.Has("wait"))
	assert.True(t, r.Has("human_input"))
	assert.True(t, r.Has("domain_agent_run"))
	assert.True(t, r.Has("sub_flow"))
	assert.True(t, r.Has("transform"))
	assert.Equal(t, 10, len(r.Types()))
	assert.NotNil(t, r.Get("skill_call"))
	assert.Nil(t, r.Get("nonexistent"))
}

func TestRegistryPanicOnDuplicate(t *testing.T) {
	r := NewRegistry()
	assert.Panics(t, func() {
		r.Register("skill_call", &SkillCallRunner{})
	})
}



func TestSkillCallRunner_Valid(t *testing.T) {
	caller := &mockSkillCaller{result: "validated", err: nil}
	input := defaultInput()
	input.Config = map[string]any{
		"skill_slug": "validate-email",
		"params":     map[string]any{"email": "test@example.com"},
	}
	input.SkillCaller = caller

	r := &SkillCallRunner{}
	out, err := r.Run(context.Background(), input)
	require.NoError(t, err)

	resultMap, ok := out.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "validated", resultMap["result"])

	require.Len(t, caller.calls, 1)
	assert.Equal(t, "validate-email", caller.calls[0].slug)
	assert.Equal(t, "test@example.com", caller.calls[0].args["email"])
}

func TestSkillCallRunner_MissingSlug(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{}
	input.SkillCaller = &mockSkillCaller{}

	r := &SkillCallRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "skill_slug required")
}

func TestSkillCallRunner_NoCaller(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{"skill_slug": "test"}


	r := &SkillCallRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SkillCaller not configured")
}

func TestSkillCallRunner_CallerError(t *testing.T) {
	caller := &mockSkillCaller{err: errors.New("skill not found")}
	input := defaultInput()
	input.Config = map[string]any{"skill_slug": "missing"}
	input.SkillCaller = caller

	r := &SkillCallRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "skill not found")
}



func TestLLMCallRunner_ValidWithTemplate(t *testing.T) {
	provider := &mockLLMProvider{result: "Respuesta: este es un resumen."}
	input := defaultInput()
	input.Config = map[string]any{
		"prompt_template": "Resume el texto: {{input.text}}",
		"model":           "gpt-4",
	}
	input.Inputs = map[string]any{"text": "Texto largo para resumir"}
	input.LLMProvider = provider

	r := &LLMCallRunner{}
	out, err := r.Run(context.Background(), input)
	require.NoError(t, err)

	resultMap, ok := out.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Respuesta: este es un resumen.", resultMap["result"])
	assert.Equal(t, "gpt-4", resultMap["model"])

	require.Len(t, provider.calls, 1)
	assert.Equal(t, "gpt-4", provider.calls[0].model)
	assert.Equal(t, "Resume el texto: Texto largo para resumir", provider.calls[0].prompt)
}

func TestLLMCallRunner_WithTemperatureAndMaxTokens(t *testing.T) {
	provider := &mockLLMProvider{result: "ok"}
	input := defaultInput()
	input.Config = map[string]any{
		"prompt_template": "Hola",
		"model":           "gpt-4",
		"temperature":     0.5,
		"max_tokens":      1000,
	}
	input.LLMProvider = provider

	r := &LLMCallRunner{}
	_, err := r.Run(context.Background(), input)
	require.NoError(t, err)

	require.Len(t, provider.calls, 1)
	assert.Equal(t, 0.5, provider.calls[0].opts["temperature"])
	assert.Equal(t, 1000, provider.calls[0].opts["max_tokens"])
}

func TestLLMCallRunner_MissingPrompt(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{"model": "gpt-4"}

	r := &LLMCallRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prompt_template required")
}

func TestLLMCallRunner_MissingModel(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{"prompt_template": "Hello"}

	r := &LLMCallRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model required")
}

func TestLLMCallRunner_NoProvider(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{
		"prompt_template": "Hello",
		"model":           "gpt-4",
	}


	r := &LLMCallRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LLMProvider not configured")
}



func TestCodeExecRunner_Stub(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{"script": "return 42"}

	r := &CodeExecRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sandbox not implemented")
}

func TestCodeExecRunner_MissingScript(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{}

	r := &CodeExecRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "script required")
}



func TestConditionalRunner_IfBranch(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{
		"condition":  "steps.s1.result.status == 'approved'",
		"if_branch":  []any{map[string]any{"id": "s3a", "type": "skill_call", "params": map[string]any{"skill_slug": "send-welcome"}}},
		"else_branch": []any{map[string]any{"id": "s3b", "type": "human_input"}},
	}
	input.StepOutputs = map[string]any{
		"s1": map[string]any{"result": map[string]any{"status": "approved"}},
	}

	r := &ConditionalRunner{}
	out, err := r.Run(context.Background(), input)
	require.NoError(t, err)

	resultMap, ok := out.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "if", resultMap["branch"])
	assert.Equal(t, true, resultMap["condition_result"])
}

func TestConditionalRunner_ElseBranch(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{
		"condition":  "steps.s1.result.status == 'approved'",
		"if_branch":  []any{map[string]any{"id": "s3a"}},
		"else_branch": []any{map[string]any{"id": "s3b"}},
	}
	input.StepOutputs = map[string]any{
		"s1": map[string]any{"result": map[string]any{"status": "rejected"}},
	}

	r := &ConditionalRunner{}
	out, err := r.Run(context.Background(), input)
	require.NoError(t, err)

	resultMap, ok := out.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "else", resultMap["branch"])
	assert.Equal(t, false, resultMap["condition_result"])
}

func TestConditionalRunner_MissingCondition(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{}

	r := &ConditionalRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "condition required")
}

func TestConditionalRunner_InvalidExpression(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{"condition": "not-a-real-expression-without-operator"}

	r := &ConditionalRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot evaluate")
}



func TestParallelRunner_ThreeBranches(t *testing.T) {
	skillCaller := &mockSkillCaller{result: "checked", err: nil}
	llmProvider := &mockLLMProvider{result: "analyzed", err: nil}

	input := defaultInput()
	input.Config = map[string]any{
		"branches": []any{
			map[string]any{"type": "skill_call", "params": map[string]any{"skill_slug": "check-email"}},
			map[string]any{"type": "llm_call", "params": map[string]any{"prompt_template": "Analyze", "model": "gpt-4"}},
		},
	}
	input.SkillCaller = skillCaller
	input.LLMProvider = llmProvider

	r := &ParallelRunner{}
	out, err := r.Run(context.Background(), input)
	require.NoError(t, err)

	resultMap, ok := out.(map[string]any)
	require.True(t, ok)

	results, ok := resultMap["results"].([]any)
	require.True(t, ok)
	assert.Len(t, results, 2)
}

func TestParallelRunner_OneBranchFails(t *testing.T) {
	skillCaller := &mockSkillCaller{result: "checked", err: nil}
	llmProvider := &mockLLMProvider{result: "", err: errors.New("LLM failed")}

	input := defaultInput()
	input.Config = map[string]any{
		"branches": []any{
			map[string]any{"type": "skill_call", "params": map[string]any{"skill_slug": "check-email"}},
			map[string]any{"type": "llm_call", "params": map[string]any{"prompt_template": "Analyze", "model": "gpt-4"}},
		},
	}
	input.SkillCaller = skillCaller
	input.LLMProvider = llmProvider

	r := &ParallelRunner{}
	out, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parallel: branch failed")

	resultMap, ok := out.(map[string]any)
	require.True(t, ok)

	errors, ok := resultMap["errors"].([]string)
	require.True(t, ok)
	assert.Equal(t, "", errors[0])
	assert.Contains(t, errors[1], "LLM failed")
}

func TestParallelRunner_EmptyBranches(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{"branches": []any{}}

	r := &ParallelRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "branches[] required")
}

func TestParallelRunner_MissingBranches(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{}

	r := &ParallelRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "branches[] required")
}



func TestWaitRunner_Duration(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{"duration_seconds": 1}

	start := time.Now()
	r := &WaitRunner{}
	out, err := r.Run(context.Background(), input)
	elapsed := time.Since(start)

	require.NoError(t, err)
	resultMap, ok := out.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "duration", resultMap["trigger"])
	assert.GreaterOrEqual(t, elapsed, 900*time.Millisecond) // some tolerance
}

func TestWaitRunner_ConditionMet(t *testing.T) {

	input := defaultInput()
	input.Config = map[string]any{
		"until_condition":          "true",
		"poll_interval_seconds":    1,
		"timeout_seconds":          5,
	}

	r := &WaitRunner{}
	out, err := r.Run(context.Background(), input)
	require.NoError(t, err)
	resultMap, ok := out.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "condition", resultMap["trigger"])
}

func TestWaitRunner_InvalidConfig(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{}

	r := &WaitRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duration_seconds or until_condition required")
}

func TestWaitRunner_BothDurationAndCondition(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{
		"duration_seconds": 10,
		"until_condition":  "true",
	}

	r := &WaitRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use either duration_seconds OR until_condition")
}



func TestHumanInputRunner_CreatesTask(t *testing.T) {
	taskCreator := &mockTaskCreator{
		taskID: "task-123",
		err:    nil,
	}

	input := defaultInput()
	input.Config = map[string]any{
		"question":      "¿Aprobar el envío?",
		"timeout_hours": 24,
	}
	input.TaskCreator = taskCreator

	r := &HumanInputRunner{}
	out, err := r.Run(context.Background(), input)
	require.NoError(t, err)

	resultMap, ok := out.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "task-123", resultMap["task_id"])
	assert.Equal(t, "awaiting_input", resultMap["status"])

	require.Len(t, taskCreator.calls, 1)
	assert.Equal(t, "¿Aprobar el envío?", taskCreator.calls[0].question)
	assert.Equal(t, 24*time.Hour, taskCreator.calls[0].timeout)
}

func TestHumanInputRunner_MissingQuestion(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{}

	r := &HumanInputRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "question required")
}

func TestHumanInputRunner_NoTaskCreator(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{
		"question": "Test question",
	}

	r := &HumanInputRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TaskCreator not configured")
}



func TestAgentRunRunner_Valid(t *testing.T) {
	agentRunner := &mockAgentRunner{
		result: map[string]any{
			"run_id": "agent-run-1",
			"status": "completed",
			"output": "Resolved issue",
		},
		err: nil,
	}

	input := defaultInput()
	input.Config = map[string]any{
		"agent_slug": "support-agent",
		"input":      "Help the user",
	}
	input.AgentRunner = agentRunner

	r := &AgentRunRunner{}
	out, err := r.Run(context.Background(), input)
	require.NoError(t, err)

	resultMap, ok := out.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "agent-run-1", resultMap["run_id"])

	require.Len(t, agentRunner.calls, 1)
	assert.Equal(t, "support-agent", agentRunner.calls[0].agentSlug)
	assert.Equal(t, "Help the user", agentRunner.calls[0].input)
}

func TestAgentRunRunner_MissingSlug(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{"input": "help"}

	r := &AgentRunRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent_slug required")
}

func TestAgentRunRunner_MissingInput(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{"agent_slug": "test-agent"}

	r := &AgentRunRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "input required")
}

func TestAgentRunRunner_TemplateInInput(t *testing.T) {
	agentRunner := &mockAgentRunner{
		result: map[string]any{"output": "done"},
		err:    nil,
	}

	input := defaultInput()
	input.Config = map[string]any{
		"agent_slug": "support-agent",
		"input":      "Help {{input.name}}",
	}
	input.Inputs = map[string]any{"name": "John"}
	input.AgentRunner = agentRunner

	r := &AgentRunRunner{}
	_, err := r.Run(context.Background(), input)
	require.NoError(t, err)

	require.Len(t, agentRunner.calls, 1)
	assert.Equal(t, "Help John", agentRunner.calls[0].input)
}



func TestSubFlowRunner_Valid(t *testing.T) {
	subRunner := &mockSubFlowRunner{
		result: "sub-flow-run-1",
		err:    nil,
	}

	input := defaultInput()
	input.Config = map[string]any{
		"flow_slug": "email-notification",
		"input":     map[string]any{"to": "user@example.com"},
	}
	input.SubFlowLauncher = subRunner

	r := &SubFlowRunner{}
	out, err := r.Run(context.Background(), input)
	require.NoError(t, err)

	resultMap, ok := out.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "sub-flow-run-1", resultMap["flow_run_id"])

	require.Len(t, subRunner.calls, 1)
	assert.Equal(t, "email-notification", subRunner.calls[0].flowSlug)
	assert.Equal(t, "user@example.com", subRunner.calls[0].inputs["to"])
}

func TestSubFlowRunner_MissingSlug(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{}

	r := &SubFlowRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "flow_slug required")
}

func TestSubFlowRunner_NoRunner(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{"flow_slug": "test-flow"}

	r := &SubFlowRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SubFlowLauncher not configured")
}



func TestTransformRunner_JSONPath(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{
		"expression": "$.s1.result.users[?(@.active==true)]",
		"engine":     "jsonpath",
	}
	input.Inputs = map[string]any{}
	input.StepOutputs = map[string]any{
		"s1": map[string]any{
			"result": map[string]any{
				"users": []any{
					map[string]any{"email": "alice@example.com", "active": true},
					map[string]any{"email": "bob@example.com", "active": false},
					map[string]any{"email": "charlie@example.com", "active": true},
				},
			},
		},
	}

	r := &TransformRunner{}
	out, err := r.Run(context.Background(), input)
	require.NoError(t, err)

	resultArr, ok := out.([]any)
	require.True(t, ok)
	assert.Len(t, resultArr, 2)
}

func TestTransformRunner_SimpleJSONPath(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{
		"expression": "$.items",
		"engine":     "jsonpath",
	}
	input.Inputs = map[string]any{
		"items": []any{1, 2, 3},
	}
	input.StepOutputs = map[string]any{}

	r := &TransformRunner{}
	out, err := r.Run(context.Background(), input)
	require.NoError(t, err)

	resultArr, ok := out.([]any)
	require.True(t, ok)
	assert.Len(t, resultArr, 3)
}

func TestTransformRunner_JQ(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{
		"expression": ".items | map(select(.price > 10))",
		"engine":     "jq",
	}
	input.Inputs = map[string]any{
		"items": []any{
			map[string]any{"name": "A", "price": 5.0},
			map[string]any{"name": "B", "price": 15.0},
			map[string]any{"name": "C", "price": 20.0},
		},
	}

	r := &TransformRunner{}
	out, err := r.Run(context.Background(), input)
	require.NoError(t, err)

	resultArr, ok := out.([]any)
	require.True(t, ok)
	assert.Len(t, resultArr, 2)
}

func TestTransformRunner_MissingExpression(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{"engine": "jsonpath"}

	r := &TransformRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expression required")
}

func TestTransformRunner_UnsupportedEngine(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{
		"expression": ".foo",
		"engine":     "unsupported",
	}

	r := &TransformRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported engine")
}



func TestResolveTemplate_Inputs(t *testing.T) {
	result := ResolveTemplate("Hello {{input.name}}, you are {{input.age}}", map[string]any{"name": "Alice", "age": 30}, nil)
	assert.Equal(t, "Hello Alice, you are 30", result)
}

func TestResolveTemplate_StepOutputs(t *testing.T) {
	outputs := map[string]any{
		"s1": map[string]any{"result": "ok"},
	}
	result := ResolveTemplate("Status: {{steps.s1.result}}", nil, outputs)
	assert.Equal(t, "Status: ok", result)
}

func TestResolveTemplate_NestedStepOutput(t *testing.T) {
	outputs := map[string]any{
		"s1": map[string]any{"result": map[string]any{"status": "approved"}},
	}
	result := ResolveTemplate("Status: {{steps.s1.result.status}}", nil, outputs)
	assert.Equal(t, "Status: approved", result)
}

func TestResolveTemplate_NoMatch(t *testing.T) {
	result := ResolveTemplate("Hello {{input.unknown}}", map[string]any{"name": "Alice"}, nil)
	assert.Equal(t, "Hello {{input.unknown}}", result)
}

func TestResolveTemplate_NoPlaceholders(t *testing.T) {
	result := ResolveTemplate("Hello World", map[string]any{"name": "Alice"}, nil)
	assert.Equal(t, "Hello World", result)
}

func TestResolveAllStrings(t *testing.T) {
	cfg := map[string]any{
		"greeting": "Hello {{input.name}}",
		"count":    42,
		"nested":   map[string]any{"inner": "{{input.foo}}"},
	}
	inputs := map[string]any{"name": "Alice"}

	resolved := ResolveAllStrings(cfg, inputs, nil)
	assert.Equal(t, "Hello Alice", resolved["greeting"])
	assert.Equal(t, 42, resolved["count"])

	nested, ok := resolved["nested"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "{{input.foo}}", nested["inner"])
}



func TestEvalBool_TrueLiteral(t *testing.T) {
	v, err := evalBool("true")
	require.NoError(t, err)
	assert.True(t, v)
}

func TestEvalBool_FalseLiteral(t *testing.T) {
	v, err := evalBool("false")
	require.NoError(t, err)
	assert.False(t, v)
}

func TestEvalBool_Equality(t *testing.T) {
	v, err := evalBool("'hello' == 'hello'")
	require.NoError(t, err)
	assert.True(t, v)

	v, err = evalBool("'hello' == 'world'")
	require.NoError(t, err)
	assert.False(t, v)
}

func TestEvalBool_NotEqual(t *testing.T) {
	v, err := evalBool("'hello' != 'world'")
	require.NoError(t, err)
	assert.True(t, v)
}

func TestEvalBool_NumericComparison(t *testing.T) {
	v, err := evalBool("5 > 3")
	require.NoError(t, err)
	assert.True(t, v)

	v, err = evalBool("3 > 5")
	require.NoError(t, err)
	assert.False(t, v)
}

func TestEvalBool_Invalid(t *testing.T) {
	_, err := evalBool("no-operator")
	require.Error(t, err)
}



func TestRegistryRoundTrip(t *testing.T) {
	r := NewRegistry()
	types := r.Types()
	assert.Len(t, types, 10)

	for _, typ := range types {
		runner := r.Get(typ)
		require.NotNil(t, runner, "runner for %q should not be nil", typ)
	}
}



func TestSkillCallRunner_Sabotage_NoValidation(t *testing.T) {


	input := defaultInput()
	input.Config = map[string]any{

		"params": map[string]any{"email": "test@example.com"},
	}
	input.SkillCaller = &mockSkillCaller{}

	r := &SkillCallRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err, "MUST fail when skill_slug is missing")
	assert.Contains(t, err.Error(), "skill_slug", "error MUST mention skill_slug")
}



func TestWaitRunner_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	input := defaultInput()
	input.Config = map[string]any{"duration_seconds": 10}

	r := &WaitRunner{}
	_, err := r.Run(ctx, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "canceled")
}



func TestHumanInputRunner_TemplateInQuestion(t *testing.T) {
	taskCreator := &mockTaskCreator{
		taskID: "task-456",
	}

	input := defaultInput()
	input.Config = map[string]any{
		"question": "¿Aprobar el envío a {{input.client}}?",
	}
	input.Inputs = map[string]any{"client": "Acme Corp"}
	input.TaskCreator = taskCreator

	r := &HumanInputRunner{}
	out, err := r.Run(context.Background(), input)
	require.NoError(t, err)

	resultMap, ok := out.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "¿Aprobar el envío a Acme Corp?", resultMap["question"])
}



func TestParallelRunner_ConfigPreservesContext(t *testing.T) {
	skillCaller := &mockSkillCaller{result: "done"}

	input := defaultInput()
	input.Config = map[string]any{
		"branches": []any{
			map[string]any{"type": "skill_call", "params": map[string]any{"skill_slug": "test"}},
		},
	}
	input.SkillCaller = skillCaller

	r := &ParallelRunner{}
	out, err := r.Run(context.Background(), input)
	require.NoError(t, err)

	resultMap, ok := out.(map[string]any)
	require.True(t, ok)
	results := resultMap["results"].([]any)
	assert.Len(t, results, 1)
}



func TestConditionalRunner_NumericCondition(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{
		"condition": "5 > 3",
	}

	r := &ConditionalRunner{}
	out, err := r.Run(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, "if", out.(map[string]any)["branch"])
}



func TestSubFlowRunner_WithTemplates(t *testing.T) {
	subRunner := &mockSubFlowRunner{
		result: "run-abc",
	}

	input := defaultInput()
	input.Config = map[string]any{
		"flow_slug": "notify-{{input.channel}}",
		"input":     map[string]any{"msg": "Hello {{input.name}}"},
	}
	input.Inputs = map[string]any{"channel": "email", "name": "Alice"}
	input.SubFlowLauncher = subRunner



	r := &SubFlowRunner{}
	out, err := r.Run(context.Background(), input)
	require.NoError(t, err)
	assert.NotNil(t, out)
	assert.Equal(t, "run-abc", out.(map[string]any)["flow_run_id"])
}



func TestWaitRunner_DurationWithContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	input := defaultInput()
	input.Config = map[string]any{"duration_seconds": 5}

	r := &WaitRunner{}
	_, err := r.Run(ctx, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}



func TestParallelRunner_UnknownBranchType(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{
		"branches": []any{
			map[string]any{"type": "nonexistent", "params": map[string]any{}},
		},
	}

	r := &ParallelRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown type")
}



func TestTransformRunner_JSONPathArrayIndex(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{
		"expression": "$.steps.s1.result.users[0].email",
		"engine":     "jsonpath",
	}
	input.StepOutputs = map[string]any{
		"s1": map[string]any{
			"result": map[string]any{
				"users": []any{
					map[string]any{"email": "alice@example.com", "active": true},
				},
			},
		},
	}

	r := &TransformRunner{}
	out, err := r.Run(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", out)
}



func TestTransformRunner_JSONPathWildcard(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{
		"expression": "$.items[*].name",
		"engine":     "jsonpath",
	}
	input.Inputs = map[string]any{
		"items": []any{
			map[string]any{"name": "A", "price": 10},
			map[string]any{"name": "B", "price": 20},
		},
	}

	r := &TransformRunner{}
	out, err := r.Run(context.Background(), input)
	require.NoError(t, err)
	arr, ok := out.([]any)
	require.True(t, ok)
	assert.Len(t, arr, 2)
}



func TestConfigHelpers(t *testing.T) {
	cfg := map[string]any{
		"str":   "hello",
		"num":   float64(42),
		"map":   map[string]any{"key": "val"},
		"slice": []any{1, 2, 3},
	}
	assert.Equal(t, "hello", configString(cfg, "str"))
	assert.Equal(t, "", configString(cfg, "missing"))
	assert.Equal(t, 42, configInt(cfg, "num"))
	assert.Equal(t, 0, configInt(cfg, "missing"))
	assert.Equal(t, map[string]any{"key": "val"}, configMap(cfg, "map"))
	assert.Equal(t, map[string]any(nil), configMap(cfg, "missing"))
	assert.Equal(t, []any{1, 2, 3}, configSlice(cfg, "slice"))
	assert.Equal(t, []any(nil), configSlice(cfg, "missing"))
}



func TestRegistry_AllTypesPresent(t *testing.T) {
	r := NewRegistry()
	expected := map[string]bool{
		"skill_call": true, "llm_call": true, "code_exec": true,
		"conditional": true, "parallel": true, "wait": true,
		"human_input": true, "domain_agent_run": true,
		"sub_flow": true, "transform": true,
	}
	for _, typ := range r.Types() {
		assert.True(t, expected[typ], "unexpected type: %s", typ)
	}
}



func TestLLMCallRunner_StepOutputsInPrompt(t *testing.T) {
	provider := &mockLLMProvider{result: "analysis done"}
	input := defaultInput()
	input.Config = map[string]any{
		"prompt_template": "Previous result: {{steps.s1.result}}",
		"model":           "gpt-4",
	}
	input.StepOutputs = map[string]any{
		"s1": map[string]any{"result": "extracted data"},
	}
	input.LLMProvider = provider

	r := &LLMCallRunner{}
	_, err := r.Run(context.Background(), input)
	require.NoError(t, err)

	require.Len(t, provider.calls, 1)
	assert.Contains(t, provider.calls[0].prompt, "extracted data")
}

func TestEvalBool_NumberComparisonWithStrings(t *testing.T) {
	v, err := evalBool("10 >= 5")
	require.NoError(t, err)
	assert.True(t, v)

	v, err = evalBool("2 <= 1")
	require.NoError(t, err)
	assert.False(t, v)
}



func TestWaitRunner_ConditionFromStepOutput(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{
		"until_condition":       "{{steps.s1.status}} == ready",
		"poll_interval_seconds": 1,
		"timeout_seconds":       3,
	}
	input.StepOutputs = map[string]any{
		"s1": map[string]any{"status": "ready"},
	}

	r := &WaitRunner{}
	out, err := r.Run(context.Background(), input)
	require.NoError(t, err)
	resultMap, ok := out.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "condition", resultMap["trigger"])
}



func TestParallelRunner_OrderedErrors(t *testing.T) {
	skillCaller := &mockSkillCaller{err: errors.New("skill failed")}
	llmProvider := &mockLLMProvider{result: "ok"}

	input := defaultInput()
	input.Config = map[string]any{
		"branches": []any{
			map[string]any{"type": "skill_call", "params": map[string]any{"skill_slug": "fail"}},
			map[string]any{"type": "llm_call", "params": map[string]any{"prompt_template": "Hi", "model": "gpt-4"}},
		},
	}
	input.SkillCaller = skillCaller
	input.LLMProvider = llmProvider

	r := &ParallelRunner{}
	out, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "skill failed")

	resultMap, ok := out.(map[string]any)
	require.True(t, ok)
	errs := resultMap["errors"].([]string)
	assert.Contains(t, errs[0], "skill failed")
	assert.Equal(t, "", errs[1])
}



func TestTransformRunner_JQSimpleSelect(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{
		"expression": ".users[]",
		"engine":     "jq",
	}
	input.Inputs = map[string]any{
		"users": []any{
			map[string]any{"name": "Alice"},
			map[string]any{"name": "Bob"},
		},
	}

	r := &TransformRunner{}
	out, err := r.Run(context.Background(), input)
	require.NoError(t, err)
	arr, ok := out.([]any)
	require.True(t, ok)
	assert.Len(t, arr, 2)
}

func TestConditionalRunner_ResolvedCondition(t *testing.T) {
	input := defaultInput()
	input.Config = map[string]any{
		"condition": "{{steps.s1.status}} == approved",
	}
	input.StepOutputs = map[string]any{
		"s1": map[string]any{"status": "approved"},
	}

	r := &ConditionalRunner{}
	out, err := r.Run(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, "if", out.(map[string]any)["branch"])
}

func TestHumanInputRunner_DefaultTimeout(t *testing.T) {
	taskCreator := &mockTaskCreator{taskID: "t1"}
	input := defaultInput()
	input.Config = map[string]any{
		"question": "Approve?",
	}
	input.TaskCreator = taskCreator

	r := &HumanInputRunner{}
	_, err := r.Run(context.Background(), input)
	require.NoError(t, err)

	require.Len(t, taskCreator.calls, 1)
	assert.Equal(t, 48*time.Hour, taskCreator.calls[0].timeout)
}

func TestAgentRunRunner_AgentRunnerError(t *testing.T) {
	agentRunner := &mockAgentRunner{err: errors.New("agent execution failed")}
	input := defaultInput()
	input.Config = map[string]any{
		"agent_slug": "test-agent",
		"input":      "help",
	}
	input.AgentRunner = agentRunner

	r := &AgentRunRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent execution failed")
}

func TestSubFlowRunner_SubFlowError(t *testing.T) {
	subRunner := &mockSubFlowRunner{err: errors.New("flow not found")}
	input := defaultInput()
	input.Config = map[string]any{
		"flow_slug": "missing-flow",
	}
	input.SubFlowLauncher = subRunner

	r := &SubFlowRunner{}
	_, err := r.Run(context.Background(), input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "flow not found")
}
