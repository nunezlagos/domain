package agentrunner

import (
	"context"
	"strings"

	"github.com/google/uuid"

	agentsvc "nunezlagos/domain/internal/service/agent"
)

// ACPTurn es el contrato mínimo de una sesión ACP nativa: un prompt one-shot
// (el agente accede a las tools reales del MCP) y cierre de recursos. Lo
// satisface el Process de internal/agentbridge/acp. Definido en el consumidor
// para no acoplar el runner al SDK de ACP; exportado solo para que el wiring
// (cmd/domain) pueda proveer el factory.
type ACPTurn interface {
	Prompt(ctx context.Context, text string) (string, error)
	Close() error
}

// ACPNativeFactory produce una sesión ACP nativa atada al runCtx del run en
// curso. El ctx recibido es el de Run (nunca el boot ctx): su cancelación mata
// el subproceso opencode.
type ACPNativeFactory func(ctx context.Context) (ACPTurn, error)

// usesNativeACP decide si Run toma el path nativo. nil = legacy tool-loop
// intacto (backward-compat).
func (r *Runner) usesNativeACP() bool {
	return r.ACPNative != nil
}

// runNativeACP ejecuta el agente vía la sesión ACP nativa. Un turno one-shot:
// compone el prompt, corre el agente (que usa las tools del MCP por su cuenta)
// y persiste el resultado. Falla estructurado si el spawn o el prompt fallan.
func (r *Runner) runNativeACP(ctx context.Context, in RunInput, ro runOpts, agent *agentsvc.Agent, orgID uuid.UUID) (*RunResult, error) {
	turn, err := r.ACPNative(ctx)
	if err != nil {
		return r.failedRun(ctx, orgID, in, ro, "acp_spawn", err)
	}
	defer func() { _ = turn.Close() }()

	text, perr := turn.Prompt(ctx, nativePrompt(agent.SystemPrompt, in.UserPrompt))
	if perr != nil {
		return r.failedRun(ctx, orgID, in, ro, "acp_prompt", perr)
	}
	return r.completeNativeRun(ctx, orgID, in, ro, agent, text)
}

// nativePrompt aplana system_prompt + user_prompt en un único texto para el
// turno ACP (el agente no recibe roles separados en este transporte).
func nativePrompt(systemPrompt, userPrompt string) string {
	var b strings.Builder
	if s := strings.TrimSpace(systemPrompt); s != "" {
		b.WriteString(s)
		b.WriteString("\n\n")
	}
	b.WriteString(strings.TrimSpace(userPrompt))
	return strings.TrimSpace(b.String())
}
