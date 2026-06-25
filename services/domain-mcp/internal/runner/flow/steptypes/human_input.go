package steptypes

import (
	"context"
	"fmt"
	"time"
)

// HumanInputRunner creates a human approval task and pauses execution.
//
// Config:
//
//	{
//	  "question": "¿Aprobar el envío al cliente {{input.client}}?",
//	  "timeout_hours": 48,
//	  "assignees": ["user-1", "user-2"]
//	}
//
// Output: {"task_id": "<uuid>", "status": "awaiting_input", "question": "<resolved>"}.
//
// The task is completed asynchronously via an external callback/webhook.
// The flow runner polls or receives a signal to resume.
type HumanInputRunner struct{}

func (r *HumanInputRunner) Run(ctx context.Context, input RunInput) (any, error) {
	question := configString(input.Config, "question")
	if question == "" {
		return nil, fmt.Errorf("human_input: question required")
	}


	question = ResolveTemplate(question, input.Inputs, input.StepOutputs)

	timeoutHours := configInt(input.Config, "timeout_hours")
	if timeoutHours <= 0 {
		timeoutHours = 48 // default 48 hours
	}
	timeout := time.Duration(timeoutHours) * time.Hour

	if input.TaskCreator == nil {

		return map[string]any{
			"task_id":  "simulated",
			"status":   "awaiting_input",
			"question": question,
			"timeout":  timeoutHours,
		}, fmt.Errorf("human_input: TaskCreator not configured (simulated task)")
	}

	taskID, err := input.TaskCreator.CreateTask(ctx, input.OrgID, question, timeout, input.Config)
	if err != nil {
		return nil, fmt.Errorf("human_input: create task: %w", err)
	}

	return map[string]any{
		"task_id":  taskID,
		"status":   "awaiting_input",
		"question": question,
		"timeout":  timeoutHours,
	}, nil
}
