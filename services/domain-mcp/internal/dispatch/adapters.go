// Adapters de producción para el Dispatcher — issue-35.1. Conectan el
// paquete dispatch (puro, sin deps de DB) con los runners reales
// (flow, agent, skill) que sí tienen deps de DB / LLM.
//
// Esto se hace en un archivo separado del paquete dispatch (sub-pkg
// adapters/) para mantener dispatch puro. Pero como dispatch ya está
// en internal/dispatch, podemos poner los adapters acá sin romper
// el layering — siempre y cuando NO se importen desde
// internal/dispatch/dispatcher.go.
package dispatch

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	agentrunner "nunezlagos/domain/internal/runner/agent"
	flowrunner "nunezlagos/domain/internal/runner/flow"
	skillrunner "nunezlagos/domain/internal/runner/skill"
	agentsvc "nunezlagos/domain/internal/service/agent"
	skillsvc "nunezlagos/domain/internal/service/skill"
)

// Adapters tiene los runners reales. Cada método devuelve un RunFunc
// que el Dispatcher puede llamar.
type Adapters struct {
	FlowRunner   *flowrunner.Runner
	AgentRunner  *agentrunner.Runner
	SkillRunner  *skillrunner.Runner
	Agents       *agentsvc.Service  // para resolver skill por ID (skill dispatch)
	Skills       *skillsvc.Service  // para resolver skill por ID
}

// RunFlowForDispatcher devuelve un RunFunc que envuelve flowRunner.Run.
func (a *Adapters) RunFlowForDispatcher() RunFunc {
	return func(ctx context.Context, req Request) (Result, error) {
		if a.FlowRunner == nil {
			return Result{}, ErrRunnerNotConfigured
		}
		inputs := mapFromJSONFlexible(req.Inputs)
		var triggeredBy *uuid.UUID
		if req.TriggeredBy != nil {
			triggeredBy = req.TriggeredBy
		}
		res, err := a.FlowRunner.Run(ctx, flowrunner.RunInput{
			FlowID: req.TargetID, TriggeredBy: triggeredBy,
			TriggerType: req.Source, Inputs: inputs,
		})
		if err != nil {
			return Result{}, err
		}
		out, _ := json.Marshal(res.Outputs)
		return Result{RunID: res.RunID, Status: res.Status, Output: out}, nil
	}
}

// RunAgentForDispatcher devuelve un RunFunc que envuelve agentRunner.Run.
func (a *Adapters) RunAgentForDispatcher() RunFunc {
	return func(ctx context.Context, req Request) (Result, error) {
		if a.AgentRunner == nil {
			return Result{}, ErrRunnerNotConfigured
		}
		inputs := mapFromJSONFlexible(req.Inputs)
		input, _ := inputs["input"].(string)
		res, err := a.AgentRunner.Run(ctx, agentrunner.RunInput{
			AgentID: req.TargetID, UserID: req.TriggeredBy,
			UserPrompt: input, Variables: inputs,
		})
		if err != nil {
			return Result{}, err
		}
		return Result{RunID: res.RunID, Status: res.Status, Output: json.RawMessage(res.Output)}, nil
	}
}

// RunSkillForDispatcher devuelve un RunFunc que envuelve skillRunner.Execute.
// Nota: el caller (cron) carga el skill por ID. En el dispatcher
// también: el TargetID es el skill_id y se carga vía Skills.GetByID.
func (a *Adapters) RunSkillForDispatcher() RunFunc {
	return func(ctx context.Context, req Request) (Result, error) {
		if a.SkillRunner == nil || a.Skills == nil {
			return Result{}, ErrRunnerNotConfigured
		}
		sk, err := a.Skills.GetByID(ctx, req.TargetID)
		if err != nil {
			return Result{}, err
		}
		inputs := mapFromJSONFlexible(req.Inputs)
		out, err := a.SkillRunner.Execute(ctx, sk, inputs)
		if err != nil {
			return Result{}, err
		}


		execID := uuid.New()
		return Result{RunID: execID, Status: "completed", Output: json.RawMessage(out)}, nil
	}
}

// ErrRunnerNotConfigured se retorna cuando un RunFunc se invoca pero
// el runner real es nil (no fue inyectado al boot).
var ErrRunnerNotConfigured = &runnerNotConfiguredError{}

type runnerNotConfiguredError struct{}

func (e *runnerNotConfiguredError) Error() string {
	return "dispatcher: runner not configured"
}

// mapFromJSONFlexible deserializa un json.RawMessage a map. A diferencia
// de mapFromJSON (en el test), tolera que sea nil o inválido.
func mapFromJSONFlexible(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
}
