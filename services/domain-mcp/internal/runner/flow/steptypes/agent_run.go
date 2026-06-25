package steptypes

import (
	"context"
	"fmt"
)

// AgentRunRunner delegates execution to the agent system.
//
// Config:
//
//	{"agent_slug": "support-agent", "input": "Help {{input.name}} with their issue"}
//
// Output: {"agent_run_id": "<uuid>", "status": "completed", "output": "<result>", ...}.
type AgentRunRunner struct{}

func (r *AgentRunRunner) Run(ctx context.Context, input RunInput) (any, error) {
	agentSlug := configString(input.Config, "agent_slug")
	if agentSlug == "" {
		return nil, fmt.Errorf("domain_agent_run: agent_slug required")
	}

	agentInput := configString(input.Config, "input")
	if agentInput == "" {
		return nil, fmt.Errorf("domain_agent_run: input required")
	}


	agentInput = ResolveTemplate(agentInput, input.Inputs, input.StepOutputs)

	if input.AgentRunner == nil {
		return nil, fmt.Errorf("domain_agent_run: AgentRunner not configured")
	}

	result, err := input.AgentRunner.Run(ctx, input.OrgID, agentSlug, agentInput, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("domain_agent_run %q: %w", agentSlug, err)
	}


	if resultMap, ok := result.(map[string]any); ok {
		return resultMap, nil
	}

	return map[string]any{
		"result": result,
		"status": "completed",
	}, nil
}
