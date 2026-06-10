package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Supervisor implementa HU-08.6: un agent "supervisor" recibe una tarea
// compleja, la descompone en sub-tasks, las asigna a sub-agents, y agrega
// resultados.
type Supervisor struct {
	Conductor       Conductor
	SupervisorSlug  string
	WorkerSlugs     []string // pool de workers disponibles
	MaxIterations   int      // límite de loops supervisor → workers → supervisor
}

// SupervisorPlan es el output que el supervisor produce decomponiendo
// la tarea. Schema esperado en la respuesta del LLM.
type SupervisorPlan struct {
	Subtasks []SubtaskAssignment `json:"subtasks"`
	Done     bool                `json:"done"`         // true cuando agregó resultado final
	Final    string              `json:"final_output,omitempty"`
}

// SubtaskAssignment.
type SubtaskAssignment struct {
	WorkerSlug  string          `json:"worker"`
	Description string          `json:"description"`
	Input       json.RawMessage `json:"input,omitempty"`
}

// Run ejecuta el loop supervisor-workers.
func (s *Supervisor) Run(ctx context.Context, initialPrompt string) (*OrchestrationResult, error) {
	res := &OrchestrationResult{Pattern: PatternSupervisor, StartedAt: time.Now()}
	maxIter := s.MaxIterations
	if maxIter <= 0 {
		maxIter = 5
	}

	currentContext := initialPrompt
	for iter := 0; iter < maxIter; iter++ {
		// 1. Supervisor planifica.
		now := time.Now()
		supTask := Task{
			ID:            uuid.New(),
			AssignedAgent: s.SupervisorSlug,
			Description:   fmt.Sprintf("supervise iteration %d", iter+1),
			Input:         []byte(currentContext),
			Status:        "running",
			StartedAt:     &now,
		}
		planJSON, _, err := s.Conductor.RunAgent(ctx, s.SupervisorSlug, supTask)
		done := time.Now()
		supTask.CompletedAt = &done
		if err != nil {
			supTask.Status = "failed"
			supTask.Error = err.Error()
			res.Tasks = append(res.Tasks, supTask)
			res.Error = err.Error()
			res.CompletedAt = time.Now()
			return res, err
		}

		var plan SupervisorPlan
		if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
			// LLM no devolvió JSON parseable — interpretar como resultado final.
			supTask.Status = "done"
			res.Tasks = append(res.Tasks, supTask)
			res.FinalOutput = planJSON
			res.Successful = true
			res.CompletedAt = time.Now()
			return res, nil
		}
		supTask.Status = "done"
		supTask.Result, _ = json.Marshal(plan)
		res.Tasks = append(res.Tasks, supTask)

		if plan.Done {
			res.FinalOutput = plan.Final
			res.Successful = true
			res.CompletedAt = time.Now()
			return res, nil
		}

		// 2. Ejecutar subtasks (cada una sub-task del supervisor parent).
		results := make([]string, 0, len(plan.Subtasks))
		for _, sub := range plan.Subtasks {
			worker := sub.WorkerSlug
			if worker == "" && len(s.WorkerSlugs) > 0 {
				worker = s.WorkerSlugs[0]
			}
			tNow := time.Now()
			t := Task{
				ID:            uuid.New(),
				Parent:        &supTask.ID,
				AssignedAgent: worker,
				Description:   sub.Description,
				Input:         sub.Input,
				Status:        "running",
				StartedAt:     &tNow,
			}
			out, result, err := s.Conductor.RunAgent(ctx, worker, t)
			tDone := time.Now()
			t.CompletedAt = &tDone
			if err != nil {
				t.Status = "failed"
				t.Error = err.Error()
				res.Tasks = append(res.Tasks, t)
				continue
			}
			t.Status = "done"
			t.Result = result
			res.Tasks = append(res.Tasks, t)
			results = append(results, fmt.Sprintf("[%s] %s", worker, out))
		}

		// 3. Re-feed context al supervisor.
		currentContext = initialPrompt + "\n\n## Resultados parciales:\n" +
			fmt.Sprintf("%v", results)
	}

	res.CompletedAt = time.Now()
	res.Successful = true
	res.FinalOutput = "max iterations reached"
	return res, nil
}
