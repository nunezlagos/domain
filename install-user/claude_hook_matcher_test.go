package main

import (
	"strings"
	"testing"
)

// DOMAINSERV-115: el marker local del gate SDD lo escribe domain-post-orchestrate.sh,
// y el token HMAC que persiste vive 30 minutos. Si el matcher no incluye las tools de
// renovación, un flow que dura más que el TTL deja al agente sin vía legítima: el flow
// sigue vivo server-side pero el gate pre-edit deniega, y la única salida es
// re-orquestar — que la policy sdd-auto-trigger prohíbe explícitamente.
//
// Se verificó en una sesión real que llamar domain_flow_grant_token emite el token
// (expires_in=1800) pero NO desbloquea la edición, porque sin la tool en el matcher el
// hook nunca corre y el marker nunca se escribe.

func postOrchestrateMatcher(t *testing.T) string {
	t.Helper()
	for _, h := range claudeHooks {
		if h.Script == "domain-post-orchestrate.sh" {
			return h.Matcher
		}
	}
	t.Fatal("no hay spec de hook para domain-post-orchestrate.sh")
	return ""
}

func TestClaudeHooks_PostOrchestrateMatcher_ToolsDeRenovacion_EstanIncluidas(t *testing.T) {
	matcher := postOrchestrateMatcher(t)

	// sin estas dos no hay forma de renovar el marker cuando vence el TTL
	for _, tool := range []string{"flow_grant_token", "flow_status"} {
		if !strings.Contains(matcher, tool) {
			t.Errorf("el matcher no incluye %q — un flow más largo que el TTL de 30min queda sin vía de renovación: %s", tool, matcher)
		}
	}
}

func TestClaudeHooks_PostOrchestrateMatcher_ToolsQueMarcanFlow_EstanIncluidas(t *testing.T) {
	matcher := postOrchestrateMatcher(t)

	// orchestrate abre el flow; phase_result/confirm lo avanzan; flow_cancel borra el marker
	for _, tool := range []string{"orchestrate", "orchestrate_phase_result", "orchestrate_confirm", "flow_cancel"} {
		if !strings.Contains(matcher, tool) {
			t.Errorf("el matcher perdió %q: %s", tool, matcher)
		}
	}
}
