package orchestration

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ParallelFanout implementa HU-08.8: lanza N agents en paralelo con
// inputs distintos (o iguales), espera todos, agrega.
type ParallelFanout struct {
	Conductor    Conductor
	Tasks        []FanoutTask
	Concurrency  int // 0 = sin limit (cuidado N grande)
}

// FanoutTask describe una unidad del fanout.
type FanoutTask struct {
	AgentSlug   string
	Description string
	Input       json.RawMessage
}

// Run ejecuta tasks en paralelo.
func (p *ParallelFanout) Run(ctx context.Context) (*OrchestrationResult, error) {
	res := &OrchestrationResult{
		Pattern:   PatternParallelFanout,
		StartedAt: time.Now(),
	}

	concurrency := p.Concurrency
	if concurrency <= 0 {
		concurrency = len(p.Tasks)
	}
	sem := make(chan struct{}, concurrency)

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		results  = make([]Task, len(p.Tasks))
		hadError bool
	)

	for i, ft := range p.Tasks {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, ft FanoutTask) {
			defer wg.Done()
			defer func() { <-sem }()

			now := time.Now()
			t := Task{
				ID:            uuid.New(),
				AssignedAgent: ft.AgentSlug,
				Description:   ft.Description,
				Input:         ft.Input,
				Status:        "running",
				StartedAt:     &now,
			}
			out, result, err := p.Conductor.RunAgent(ctx, ft.AgentSlug, t)
			done := time.Now()
			t.CompletedAt = &done
			if err != nil {
				t.Status = "failed"
				t.Error = err.Error()
				mu.Lock()
				hadError = true
				mu.Unlock()
			} else {
				t.Status = "done"
				t.Result = result
				_ = out // could be summary
			}
			results[idx] = t
		}(i, ft)
	}
	wg.Wait()

	res.Tasks = results
	res.CompletedAt = time.Now()
	res.Successful = !hadError
	return res, nil
}
