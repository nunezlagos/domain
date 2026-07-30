package phases

import (
	"context"
	"testing"
)

// DOMAINSERV-208: SubagentPlans() exponía 1 de los 3 planes que existían. Los de sdd-4r y
// sdd-verify no llegaban a agent_templates.metadata.subagent_plan, así que el seeder los
// ignoraba y los guards de catálogo tampoco los miraban — con DOMAINSERV-180, que creó ese
// mecanismo, en `done`.
//
// El hueco no se veía leyendo ninguno de los dos lados: SubagentPlans() se ve completo si no
// sabés cuántos planes hay, y cada fase se ve bien porque su plan sí existe. Solo aparece al
// cruzarlos, que es lo que hace este guard.

// handlersConPlanPotencial son todas las fases del pipeline. La lista está duplicada del
// wiring de cmd/domain-mcp/main.go:223-234 y cmd/domain/init_cli.go:294+, que ya eran dos
// copias entre sí: unificarlas en un AllHandlers() del paquete es trabajo aparte y queda
// registrado como tal. Mientras siga duplicada, agregar una fase nueva exige agregarla acá
// o este guard no la mira — de ahí el nombre explícito.
func handlersConPlanPotencial() []Handler {
	return []Handler{
		NewSDDExploreHandler(),
		NewSDDSpecHandler(),
		NewSDDProposeHandler(),
		NewSDDDesignHandler(),
		NewSDDTasksHandler(),
		NewSDDApplyHandler(),
		NewSDDVerifyHandler(),
		NewSDDJudgeHandler(),
		NewSDD4RHandler(),
		NewSDDReviewHandler(),
		NewSDDArchiveHandler(),
		NewSDDOnboardHandler(),
	}
}

// buildParaGuard construye la fase y FALLA el test si Build devuelve error. El error no se
// saltea a propósito: la primera versión de este guard hacía `continue` ante error, todas las
// fases fallaban con "RawText vacío", y el guard pasaba sin mirar una sola fase. Lo destapó el
// test inverso (NoRegistraPlanesHuerfanos), que vio sdd-explore registrado y "nadie" que lo
// declarara. Un guard que ante un error deja de mirar no reporta el error: reporta verde.
func buildParaGuard(t *testing.T, h Handler) *Output {
	t.Helper()
	out, err := h.Build(context.Background(), Input{
		PhaseSlug: h.Slug(),
		RawText:   "tarea de prueba del guard de registro de planes",
	})
	if err != nil {
		t.Fatalf("Build de %s devolvió error, así que el guard no puede verificar nada: %v",
			h.Slug(), err)
	}
	return out
}

func TestSubagentPlans_TodaFaseQueDeclaraUnPlanEstaRegistrada(t *testing.T) {
	registrados := SubagentPlans()

	for _, h := range handlersConPlanPotencial() {
		slug := string(h.Slug())
		out := buildParaGuard(t, h)
		if out.SubagentPlan == "" {
			continue
		}
		plan, ok := registrados[slug]
		if !ok {
			t.Errorf(
				"la fase %s declara un SubagentPlan pero NO está en SubagentPlans(): el "+
					"AgentTemplatesCatalogSeeder no lo va a sembrar a "+
					"agent_templates.metadata.subagent_plan, así que el plan existe en el "+
					"código y no llega a la BD", slug)
			continue
		}
		if plan != out.SubagentPlan {
			t.Errorf(
				"el plan de %s registrado en SubagentPlans() NO es el que la fase declara: "+
					"son dos copias del mismo texto y ya divergieron, que es exactamente el "+
					"defecto que DOMAINSERV-180 vino a cerrar", slug)
		}
	}
}

// El inverso: una entrada registrada que ninguna fase declara es un plan que se siembra a la
// BD y nadie inyecta al prompt. Se vería como funcionando desde el lado de la BD.
func TestSubagentPlans_NoRegistraPlanesHuerfanos(t *testing.T) {
	declarados := map[string]bool{}
	for _, h := range handlersConPlanPotencial() {
		if buildParaGuard(t, h).SubagentPlan != "" {
			declarados[string(h.Slug())] = true
		}
	}

	for slug, plan := range SubagentPlans() {
		if plan == "" {
			t.Errorf("SubagentPlans() registra %s con un plan vacío", slug)
		}
		if !declarados[slug] {
			t.Errorf(
				"SubagentPlans() registra %s pero ninguna fase declara ese plan: se sembraría "+
					"a la BD sin que nadie lo inyecte al prompt", slug)
		}
	}
}

// Cada agente nombrado en un plan va con MarcaFallback. El server no sabe qué catálogo tiene
// instalado el cliente, así que nombrar un agente es una preferencia; sin la marca, un cliente
// sin ese agente recibe una instrucción que no puede cumplir. Ya lo cubre el guard de
// subagent_catalog_test.go para los planes que conocía: acá se aplica a TODOS los registrados.
func TestSubagentPlans_TodoAgenteNombradoLlevaMarcaFallback(t *testing.T) {
	for slug, plan := range SubagentPlans() {
		nombrados := AgentesNombradosEn(plan)
		if len(nombrados) == 0 {
			continue
		}
		if !contieneMarca(plan) {
			t.Errorf(
				"el plan de %s nombra %v sin MarcaFallback (%q): en un cliente sin esos "+
					"agentes la instrucción no se puede cumplir",
				slug, nombrados, MarcaFallback)
		}
	}
}

func contieneMarca(plan string) bool {
	return len(plan) > 0 && len(MarcaFallback) > 0 && containsSubstr(plan, MarcaFallback)
}

func containsSubstr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
