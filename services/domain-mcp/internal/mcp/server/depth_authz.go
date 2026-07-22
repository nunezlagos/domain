package mcpserver

// DepthHeader es el header que porta la profundidad de anidamiento de agentes.
// El mcpServer que domain le expone a opencode (agent_run nativo) lo fija en 1;
// el handler HTTP lo lee para restringir el allowlist del principal.
const DepthHeader = "X-Domain-Agent-Depth"

// reentrantTools son las tools que un agent_run anidado NO puede invocar:
// spawnean/ejecutan agentes o programan ejecución diferida → riesgo de
// recursión (un agente lanzando agentes/flows/orquestaciones/crons).
// Diseño ideal: un allowlist EXPLÍCITO de tools seguras (pendiente); esto
// es el denylist endurecido interino.
var reentrantTools = map[string]bool{
	"domain_agent_run":                true,
	"domain_agent_create":             true,
	"domain_orchestrate":              true,
	"domain_orchestrate_phase_result": true,
	"domain_orchestrate_confirm":      true,
	"domain_flow_run":                 true,
	"domain_flow_create":              true,
	"domain_cron_create":              true,
	"domain_cron_set_enabled":         true,
	"domain_skill_execute":            true,
}

// AllowedToolsForDepth calcula el allowlist de un principal según la profundidad
// de anidamiento. depth<=0 → nil (full access, comportamiento actual). depth>=1
// → todas las tools registradas MENOS las reentrantes (barrera anti-reentrancia
// del path nativo ACP, DOMAINSERV-85). El enforcement lo hace ResilientWrapper
// vía toolAllowed: una lista no vacía sin el tool → deny fail-closed.
func AllowedToolsForDepth(deps Deps, depth int) []string {
	if depth <= 0 {
		return nil
	}
	tools := Tools(deps)
	allowed := make([]string, 0, len(tools))
	for _, st := range tools {
		if reentrantTools[st.Tool.Name] {
			continue
		}
		allowed = append(allowed, st.Tool.Name)
	}
	return allowed
}

// AllowedToolsForDepthScoped combina la barrera por depth con el allowlist
// preexistente del principal: depth solo puede RESTRINGIR, nunca ampliar. Si el
// token ya viene scoped (existing no vacío) devuelve la intersección con el
// allowlist de depth; si el principal no tenía allowlist (nil/vacío) devuelve el
// de depth tal cual. depth<=0 preserva existing sin tocar.
func AllowedToolsForDepthScoped(deps Deps, depth int, existing []string) []string {
	if depth <= 0 {
		return existing
	}
	byDepth := AllowedToolsForDepth(deps, depth)
	if len(existing) == 0 {
		return byDepth
	}
	return intersectTools(existing, byDepth)
}

// intersectTools devuelve los elementos de a que también están en b.
func intersectTools(a, b []string) []string {
	inB := make(map[string]bool, len(b))
	for _, s := range b {
		inB[s] = true
	}
	out := make([]string, 0, len(a))
	for _, s := range a {
		if inB[s] {
			out = append(out, s)
		}
	}
	return out
}
