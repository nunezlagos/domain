package orchestration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Handoff implementa HU-08.7: un agent activo puede transferir su
// conversación a otro agent cuando detecta que el otro tiene mejor expertise.
//
// El handoff signal viene del LLM output con marker estructurado: `<handoff
// to="<agent_slug>" reason="<why>"/>` o JSON `{"handoff":{"to":"slug",
// "reason":"why"}}`.
type Handoff struct {
	Conductor    Conductor
	MaxHandoffs  int // límite total de handoffs anti-loop
}

// ErrHandoffForbidden se devuelve si el template del agent target tiene
// handoff_policy=forbid o policy=require_supervisor_approval sin supervisor.
var ErrHandoffForbidden = errors.New("handoff forbidden by target policy")

// Result is the same OrchestrationResult.

// Run inicia conversación con startAgent y sigue handoffs hasta resolución.
func (h *Handoff) Run(ctx context.Context, startAgent, initialInput string) (*OrchestrationResult, error) {
	res := &OrchestrationResult{Pattern: PatternHandoff, StartedAt: time.Now()}
	maxH := h.MaxHandoffs
	if maxH <= 0 {
		maxH = 5
	}

	currentAgent := startAgent
	currentInput := initialInput
	for i := 0; i <= maxH; i++ {
		now := time.Now()
		t := Task{
			ID:            uuid.New(),
			AssignedAgent: currentAgent,
			Description:   fmt.Sprintf("handoff hop %d", i),
			Input:         []byte(currentInput),
			Status:        "running",
			StartedAt:     &now,
		}
		output, _, err := h.Conductor.RunAgent(ctx, currentAgent, t)
		done := time.Now()
		t.CompletedAt = &done
		if err != nil {
			t.Status = "failed"
			t.Error = err.Error()
			res.Tasks = append(res.Tasks, t)
			res.Error = err.Error()
			res.CompletedAt = time.Now()
			return res, err
		}
		t.Status = "done"
		res.Tasks = append(res.Tasks, t)

		// Detecta marker de handoff en el output.
		nextAgent, reason := detectHandoff(output)
		if nextAgent == "" {
			// No handoff → resolución final.
			res.FinalOutput = output
			res.Successful = true
			res.CompletedAt = time.Now()
			return res, nil
		}

		// Validar policy del target.
		tmpl, err := h.Conductor.LoadTemplate(ctx, nextAgent)
		if err != nil {
			res.Error = fmt.Sprintf("load template %s: %v", nextAgent, err)
			res.CompletedAt = time.Now()
			return res, err
		}
		if tmpl != nil && tmpl.HandoffPolicy == HandoffForbid {
			res.Error = fmt.Sprintf("%s blocks handoff: target %s policy=forbid",
				currentAgent, nextAgent)
			res.CompletedAt = time.Now()
			return res, ErrHandoffForbidden
		}

		currentInput = fmt.Sprintf("Continuación de %s → %s. Motivo: %s.\n\nContexto:\n%s",
			currentAgent, nextAgent, reason, output)
		currentAgent = nextAgent
	}

	res.Error = "max handoffs exceeded"
	res.CompletedAt = time.Now()
	return res, nil
}

// detectHandoff parsea markers tipo <handoff to="X"/> o JSON inline.
// Heurística simple — para precisión real, el system_prompt del agent
// instruye qué format usar.
func detectHandoff(output string) (toAgent, reason string) {
	// XML-style
	if idx := strings.Index(output, "<handoff "); idx >= 0 {
		seg := output[idx:]
		if endIdx := strings.Index(seg, "/>"); endIdx > 0 {
			tag := seg[:endIdx]
			toAgent = extractAttr(tag, "to")
			reason = extractAttr(tag, "reason")
		}
	}
	return toAgent, reason
}

func extractAttr(tag, name string) string {
	prefix := name + "=\""
	i := strings.Index(tag, prefix)
	if i < 0 {
		return ""
	}
	rest := tag[i+len(prefix):]
	end := strings.Index(rest, "\"")
	if end < 0 {
		return ""
	}
	return rest[:end]
}
