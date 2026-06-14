package orchestration

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Sequential implementa issue-08.4: encadena agents en orden, pasando output
// de uno como input del siguiente.
type Sequential struct {
	Conductor Conductor
	Steps     []SequentialStep
}

// SequentialStep describe un paso del pipeline.
type SequentialStep struct {
	AgentSlug   string
	Description string
}

// Run ejecuta el pipeline secuencial.
func (s *Sequential) Run(ctx context.Context, initialInput []byte) (*OrchestrationResult, error) {
	res := &OrchestrationResult{Pattern: PatternSequential, StartedAt: time.Now()}
	currentInput := initialInput

	for _, step := range s.Steps {
		now := time.Now()
		t := Task{
			ID:            uuid.New(),
			AssignedAgent: step.AgentSlug,
			Description:   step.Description,
			Input:         currentInput,
			Status:        "running",
			StartedAt:     &now,
		}
		output, result, err := s.Conductor.RunAgent(ctx, step.AgentSlug, t)
		done := time.Now()
		t.CompletedAt = &done
		if err != nil {
			t.Status = "failed"
			t.Error = err.Error()
			res.Tasks = append(res.Tasks, t)
			res.Error = fmt.Sprintf("step %s failed: %v", step.AgentSlug, err)
			res.CompletedAt = time.Now()
			return res, err
		}
		t.Status = "done"
		t.Result = result
		res.Tasks = append(res.Tasks, t)
		currentInput = []byte(output)
		res.FinalOutput = output
	}

	res.CompletedAt = time.Now()
	res.Successful = true
	return res, nil
}
