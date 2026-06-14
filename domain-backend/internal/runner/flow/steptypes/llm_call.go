package steptypes

import (
	"context"
	"fmt"
)

// LLMCallRunner resolves a prompt template and sends it to an LLM provider.
//
// Config:
//
//	{
//	  "prompt_template": "Resume el texto: {{input.text}}",
//	  "model": "gpt-4",
//	  "temperature": 0.5,
//	  "max_tokens": 1000
//	}
//
// Output: {"result": "<llm_response>", "model": "<model>", ...}.
type LLMCallRunner struct{}

func (r *LLMCallRunner) Run(ctx context.Context, input RunInput) (any, error) {
	promptTemplate := configString(input.Config, "prompt_template")
	if promptTemplate == "" {
		return nil, fmt.Errorf("llm_call: prompt_template required")
	}
	model := configString(input.Config, "model")
	if model == "" {
		return nil, fmt.Errorf("llm_call: model required")
	}

	// Resolve template with inputs and step outputs.
	prompt := ResolveTemplate(promptTemplate, input.Inputs, input.StepOutputs)
	if prompt == "" {
		return nil, fmt.Errorf("llm_call: resolved prompt is empty")
	}

	if input.LLMProvider == nil {
		return nil, fmt.Errorf("llm_call: LLMProvider not configured")
	}

	// Build LLM options from config (temperature, max_tokens, etc.).
	opts := make(map[string]any)
	if v, ok := input.Config["temperature"]; ok {
		opts["temperature"] = v
	}
	if v, ok := input.Config["max_tokens"]; ok {
		opts["max_tokens"] = v
	}
	if v, ok := input.Config["top_p"]; ok {
		opts["top_p"] = v
	}
	if v, ok := input.Config["stop"]; ok {
		opts["stop"] = v
	}

	result, err := input.LLMProvider.Complete(ctx, model, prompt, opts)
	if err != nil {
		return nil, fmt.Errorf("llm_call %q: %w", model, err)
	}
	return map[string]any{
		"result": result,
		"model":  model,
	}, nil
}
