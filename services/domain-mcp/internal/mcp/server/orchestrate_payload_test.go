package mcpserver

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	orchsvc "nunezlagos/domain/internal/service/orchestrator"
)

// DOMAINSERV-142: el payload de domain_orchestrate volvió a exceder el límite del tool
// result del cliente (58.532 chars / 232 líneas medidos en prod con un raw_text de
// ~4.500 chars). El flow se creaba en BD igual — la falla es de ENTREGA: el cliente
// queda sin el plan aunque el trabajo esté persistido.
//
// Desglose medido del payload que reventó, por campo:
//
//	plan.steps[0].system_prompt   37.853   (65% del total, por sí solo)
//	snapshot_prompt                5.728
//	plan.steps[0].user_prompt      5.728   (idéntico al de arriba: duplicación pura)
//	resto (11 steps + metadata)    ~9.200
//	                              -------
//	total                          58.532
//
// Este test es el guard que hacía falta: mide el payload por campo, así que la próxima
// vez que crezca se ve acá y no en producción.

// maxPayloadChars: techo del payload de transporte. El límite real del cliente ronda
// los ~25k tokens; 45k chars de markdown/JSON quedan por debajo con margen para que un
// raw_text más largo que el del repro no lo cruce.
const maxPayloadChars = 45000

// planRealista reproduce la forma del plan que reventó: 12 steps (modo full), el step 0
// con su system_prompt inline y el user_prompt derivado de un raw_text largo.
func planRealista(sysPrompt0Len, userPrompt0Len int) *orchsvc.OrchestrateResult {
	steps := make([]orchsvc.PhaseStepSummary, 12)
	slugs := []string{
		"sdd-explore", "sdd-spec", "sdd-propose", "sdd-design", "sdd-tasks",
		"sdd-apply", "sdd-verify", "sdd-review", "sdd-audit", "sdd-archive",
		"sdd-report", "sdd-close",
	}
	for i := range steps {
		steps[i] = orchsvc.PhaseStepSummary{
			Slug:              orchsvc.PhaseSlug(slugs[i]),
			AgentTemplateSlug: "agent-" + slugs[i],
			RetryPolicy:       "retry_once",
			RequiredToolCalls: []string{"domain_mem_save", "domain_orchestrate_phase_result"},
			SuggestedSaves: []orchsvc.SuggestedSaveSummary{
				{Type: "decision", Required: true, Hint: "persistir la decisión de la fase"},
			},
		}
		// exportPlan ya strippea el system_prompt de los steps 1..N (DOMAINSERV-3);
		// acá se refleja ese estado de entrada
		if i == 0 {
			steps[i].SystemPrompt = strings.Repeat("S", sysPrompt0Len)
			steps[i].UserPrompt = strings.Repeat("U", userPrompt0Len)
		}
	}
	res := &orchsvc.OrchestrateResult{
		Mode: "full",
		Plan: &orchsvc.PhasePlanSummary{Mode: "full", Steps: steps},
	}
	res.SnapshotPrompt = steps[0].UserPrompt
	return res
}

// desglose devuelve el tamaño de cada campo del payload serializado, para que el test
// reporte DÓNDE está el peso y no solo el total.
func desglose(res *orchsvc.OrchestrateResult) (map[string]int, int) {
	body, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		panic(err)
	}
	d := map[string]int{
		"snapshot_prompt": len(res.SnapshotPrompt),
	}
	if res.Plan != nil {
		for i, st := range res.Plan.Steps {
			if st.SystemPrompt != "" {
				d[fmt.Sprintf("steps[%d].system_prompt", i)] = len(st.SystemPrompt)
			}
			if st.UserPrompt != "" {
				d[fmt.Sprintf("steps[%d].user_prompt", i)] = len(st.UserPrompt)
			}
		}
	}
	return d, len(body)
}

func reportar(t *testing.T, etiqueta string, res *orchsvc.OrchestrateResult) int {
	t.Helper()
	d, total := desglose(res)
	claves := make([]string, 0, len(d))
	for k := range d {
		claves = append(claves, k)
	}
	sort.Slice(claves, func(i, j int) bool { return d[claves[i]] > d[claves[j]] })
	t.Logf("%s — total %d chars:", etiqueta, total)
	for _, k := range claves {
		if d[k] > 0 {
			t.Logf("    %-30s %7d", k, d[k])
		}
	}
	return total
}

// El repro del ticket: con los tamaños medidos en prod, el payload excede el límite.
func TestTrimOrchestrateForTransport_ReproDelBug_ExcedeSinTrim(t *testing.T) {
	res := planRealista(37853, 5728)
	total := reportar(t, "sin trim (repro DOMAINSERV-142)", res)
	assert.Greater(t, total, maxPayloadChars,
		"el repro tiene que exceder el techo, o no está reproduciendo el bug")
}

// La dedup: snapshot_prompt y steps[0].user_prompt son el mismo string; se manda uno.
func TestTrimOrchestrateForTransport_DeduplicaSnapshotPrompt(t *testing.T) {
	res := planRealista(18000, 5728)
	out := trimOrchestrateForTransport(res)

	assert.Empty(t, out.SnapshotPrompt, "snapshot_prompt no se repite")
	assert.Equal(t, strings.Repeat("U", 5728), out.Plan.Steps[0].UserPrompt,
		"el user_prompt del step 0 SÍ viaja: es el campo que la descripción de la tool "+
			"le promete al cliente ('los steps que el cliente IDE debe ejecutar')")

	// el resultado del service no se toca: el worker async y promptrouter lo siguen viendo
	assert.Equal(t, strings.Repeat("U", 5728), res.SnapshotPrompt,
		"el trim es de transporte, no muta el OrchestrateResult original")
}

// Si por alguna razón dejan de ser idénticos, NO se toca nada: el trim deduplica, no
// descarta información.
func TestTrimOrchestrateForTransport_SnapshotDistinto_NoSeToca(t *testing.T) {
	res := planRealista(1000, 500)
	res.SnapshotPrompt = "un resumen distinto del user_prompt"

	out := trimOrchestrateForTransport(res)

	assert.Equal(t, "un resumen distinto del user_prompt", out.SnapshotPrompt,
		"solo se deduplica lo que es idéntico")
	assert.NotEmpty(t, out.Plan.Steps[0].UserPrompt)
}

// Sin regresión de DOMAINSERV-3/108: los steps 1..N siguen sin prompts en el payload
// inicial (los reciben vía phase_result).
func TestTrimOrchestrateForTransport_Steps1aN_SinPrompts(t *testing.T) {
	res := planRealista(18000, 5728)
	res.Plan.Steps[3].SystemPrompt = "no debería viajar"
	res.Plan.Steps[3].UserPrompt = "tampoco"

	out := trimOrchestrateForTransport(res)

	for i := 1; i < len(out.Plan.Steps); i++ {
		assert.Empty(t, out.Plan.Steps[i].SystemPrompt, "step %d sin system_prompt", i)
		assert.Empty(t, out.Plan.Steps[i].UserPrompt, "step %d sin user_prompt", i)
	}
	// los steps posteriores conservan su metadata: el cliente necesita saber qué viene
	assert.Equal(t, "sdd-verify", string(out.Plan.Steps[6].Slug))
	assert.NotEmpty(t, out.Plan.Steps[6].RequiredToolCalls)
}

// El objetivo del ticket: con el system_prompt del step 0 acotado por el presupuesto
// del rulesBlock (~21KB: ~15KB de template base + 6KB de policies) MÁS la dedup, el
// payload entra completo en el tool result.
func TestTrimOrchestrateForTransport_ConPresupuesto_EntraEnElLimite(t *testing.T) {
	res := planRealista(21000, 5728)

	out := trimOrchestrateForTransport(res)
	total := reportar(t, "con trim + presupuesto del rulesBlock", out)

	assert.Less(t, total, maxPayloadChars,
		"el payload tiene que entrar completo: el cliente no puede quedarse sin el plan")
	require.NotEmpty(t, out.Plan.Steps[0].SystemPrompt, "el step 0 sigue ejecutable")
	require.NotEmpty(t, out.Plan.Steps[0].UserPrompt, "el step 0 sigue ejecutable")
}

// Un raw_text el doble de largo que el del repro tampoco lo cruza: el margen es real y
// no un ajuste al caso exacto que falló.
func TestTrimOrchestrateForTransport_RawTextDoble_SigueEntrando(t *testing.T) {
	res := planRealista(21000, 5728*2)

	total := reportar(t, "raw_text x2", trimOrchestrateForTransport(res))

	assert.Less(t, total, maxPayloadChars,
		"la falla era intermitente por tamaño de prompt: el margen tiene que aguantar")
}
