package steptypes

import (
	"context"
	"fmt"
)

// SubFlowRunner launches a sub-flow and waits for its result.
//
// Config:
//
//	{"flow_slug": "email-notification", "input": {"to": "{{input.email}}", "template": "welcome"}}
//
// Output: {"flow_run_id": "<uuid>", "status": "completed", "outputs": {...}}.
type SubFlowRunner struct{}

func (r *SubFlowRunner) Run(ctx context.Context, input RunInput) (any, error) {
	flowSlug := configString(input.Config, "flow_slug")
	if flowSlug == "" {
		return nil, fmt.Errorf("sub_flow: flow_slug required")
	}

	subInputs := configMap(input.Config, "input")
	if subInputs == nil {
		subInputs = map[string]any{}
	}

	if input.SubFlowLauncher == nil {
		return nil, fmt.Errorf("sub_flow: SubFlowLauncher not configured")
	}

	result, err := input.SubFlowLauncher.Run(ctx, input.OrgID, flowSlug, subInputs, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("sub_flow %q: %w", flowSlug, err)
	}

	return map[string]any{
		"flow_run_id": result,
		"status":      "completed",
	}, nil
}
