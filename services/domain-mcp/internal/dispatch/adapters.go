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
	Agents       *agentsvc.Service           // para resolver skill por ID (skill dispatch)
	Skills       *skillsvc.Service           // para resolver skill por ID
	SkillExec    *skillsvc.ExecutionService  // persiste skill_executions (HU-52.2: created_by)
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

// RunSkillForDispatcher devuelve un RunFunc que ejecuta el skill PERSISTIENDO
// la ejecución en skill_executions vía ExecutionService.Execute.
//
// HU-52.2: antes este adapter llamaba SkillRunner.Execute directo y fabricaba
// un uuid.New() como RunID, SIN insertar fila en skill_executions. Resultado:
// las ejecuciones de skills por el path real (cron/webhook/MCP) nunca quedaban
// registradas, created_by nunca se poblaba y unique_callers_count del aggregator
// quedaba clavado en 0 (el TODO de HU-52.2). Ahora se delega a
// ExecutionService.Execute, que valida, resuelve versión, inserta la fila con
// created_by = req.TriggeredBy (caller del Principal; nil en triggers de
// sistema → NULL), corre el runner y persiste el resultado.
//
// El TargetID es el skill_id; ExecutionService lo resuelve internamente.
func (a *Adapters) RunSkillForDispatcher() RunFunc {
	return func(ctx context.Context, req Request) (Result, error) {
		if a.SkillExec == nil {
			return Result{}, ErrRunnerNotConfigured
		}
		inputs := mapFromJSONFlexible(req.Inputs)
		exec, err := a.SkillExec.Execute(ctx, skillsvc.ExecuteInput{
			OrganizationID: req.OrgID,
			SkillID:        req.TargetID,
			Parameters:     inputs,
			Mode:           "sync",
			CreatedBy:      req.TriggeredBy,
		})
		if err != nil {
			return Result{}, err
		}
		var out json.RawMessage
		if exec.Output != nil {
			out = json.RawMessage(*exec.Output)
		}
		return Result{RunID: exec.ID, Status: exec.Status, Output: out}, nil
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
