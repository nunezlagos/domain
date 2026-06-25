package steptypes

import (
	"context"
	"fmt"
)

// CodeExecRunner executes a sandboxed script.
// Stub implementation until REQ-11 (sandbox runner).
//
// Config:
//
//	{"script": "return data.items.filter(i => i.active).length"}
//
// Output: {"result": <script_output>}.
type CodeExecRunner struct{}

func (r *CodeExecRunner) Run(_ context.Context, input RunInput) (any, error) {
	script := configString(input.Config, "script")
	if script == "" {
		return nil, fmt.Errorf("code_exec: script required")
	}



	return nil, fmt.Errorf("code_exec: sandbox not implemented yet (REQ-11 pending), script=%q", script)
}
