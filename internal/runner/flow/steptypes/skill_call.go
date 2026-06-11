package steptypes

import (
	"context"
	"fmt"
)

// SkillCallRunner invokes a named skill with the configured args.
//
// Config:
//
//	{"skill_slug": "validate-email", "params": {"field": "email"}}
//
// Output: {"result": <skill_output>}.
type SkillCallRunner struct{}

func (r *SkillCallRunner) Run(ctx context.Context, input RunInput) (any, error) {
	slug := configString(input.Config, "skill_slug")
	if slug == "" {
		return nil, fmt.Errorf("skill_call: skill_slug required")
	}
	params := configMap(input.Config, "params")
	if params == nil {
		params = map[string]any{}
	}

	if input.SkillCaller == nil {
		return nil, fmt.Errorf("skill_call: SkillCaller not configured")
	}

	result, err := input.SkillCaller.Call(ctx, input.OrgID, slug, params)
	if err != nil {
		return nil, fmt.Errorf("skill_call %q: %w", slug, err)
	}
	return map[string]any{"result": result}, nil
}
