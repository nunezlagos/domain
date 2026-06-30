package steptypes

import (
	"context"
	"fmt"
	"sync"
)

// ParallelRunner executes N branches concurrently and waits for all to complete.
//
// Config:
//
//	{
//	  "branches": [
//	    {"id": "p1", "type": "skill_call", "params": {"skill_slug": "check-email"}},
//	    {"id": "p2", "type": "llm_call", "params": {"prompt_template": "Analiza: {{input.text}}"}}
//	  ]
//	}
//
// Output: {"results": [...], "errors": [...]} — parallel arrays ordered by branch definition.
type ParallelRunner struct{}

func (r *ParallelRunner) Run(ctx context.Context, input RunInput) (any, error) {
	branches := configSlice(input.Config, "branches")
	if len(branches) == 0 {
		return nil, fmt.Errorf("parallel: branches[] required")
	}
	maxBranches := 32
	if len(branches) > maxBranches {
		return nil, fmt.Errorf("parallel: max %d branches, got %d", maxBranches, len(branches))
	}


	type branchStep struct {
		idx    int
		typ    string
		params map[string]any
	}

	steps := make([]branchStep, 0, len(branches))
	for i, b := range branches {
		bMap, ok := b.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("parallel branch[%d]: must be object", i)
		}
		typ, _ := bMap["type"].(string)
		params, _ := bMap["params"].(map[string]any)
		if params == nil {

			params, _ = bMap["config"].(map[string]any)
		}
		steps = append(steps, branchStep{idx: i, typ: typ, params: params})
	}

	type result struct {
		idx int
		val any
		err error
	}

	resCh := make(chan result, len(steps))
	var wg sync.WaitGroup

	for _, s := range steps {
		wg.Add(1)
		s := s // capture
		go func() {
			defer wg.Done()
			runner := r.getRunner(s.typ)
			if runner == nil {
				resCh <- result{idx: s.idx, err: fmt.Errorf("parallel branch[%d]: unknown type %q", s.idx, s.typ)}
				return
			}
			branchInput := input
			branchInput.Config = s.params
			val, err := runner.Run(ctx, branchInput)
			resCh <- result{idx: s.idx, val: val, err: err}
		}()
	}

	wg.Wait()
	close(resCh)

	results := make([]any, len(steps))
	errs := make([]string, len(steps))
	var firstErr error

	for res := range resCh {
		results[res.idx] = res.val
		if res.err != nil {
			errs[res.idx] = res.err.Error()
			if firstErr == nil {
				firstErr = res.err
			}
		}
	}

	out := map[string]any{"results": results, "errors": errs}
	if firstErr != nil {
		return out, fmt.Errorf("parallel: branch failed: %w", firstErr)
	}
	return out, nil
}

func (r *ParallelRunner) getRunner(typ string) StepRunner {
	if typ == "" {
		return nil
	}


	switch typ {
	case "skill_call":
		return &SkillCallRunner{}
	case "llm_call":
		return &LLMCallRunner{}
	case "code_exec":
		return &CodeExecRunner{}
	case "wait":
		return &WaitRunner{}
	case "transform":
		return &TransformRunner{}
	case "conditional":
		return &ConditionalRunner{}
	case "human_input":
		return &HumanInputRunner{}
	case "domain_agent_run":
		return &AgentRunRunner{}
	case "sub_flow":
		return &SubFlowRunner{}
	}
	return nil
}
