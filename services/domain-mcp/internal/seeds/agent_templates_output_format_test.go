package seeds

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-210: el agente arma su output copiando el <output_format> del prompt,
// que es lo que el prompt le ordena hacer. Cuando el validador del handler exige un
// campo que ese bloque no declara, la fase rechaza el cierre y cuesta un reintento
// evitable. Medido ejecutando el pipeline full el 2026-07-30: las tres fallaron.
//
// La ventana se acota al bloque <output_format> a propósito: un Contains sobre el
// prompt entero pasa en verde con el campo mencionado en la prosa o en el <example>,
// que es justo lo que el agente NO copia.
func TestAgentTemplates_OutputFormat_DeclaraLosCamposQueElValidadorExige(t *testing.T) {
	casos := []struct {
		slug   string
		campo  string
		motivo string
	}{
		{"sdd-propose", `"proposal_md"`, "phases/sdd_propose.go:58 rechaza el cierre sin proposal_md"},
		{"sdd-design", `"design_md"`, "phases/sdd_design.go:68 rechaza el cierre sin design_md"},
		{"sdd-tasks", `"id"`, "phases/sdd_tasks.go:74 rechaza el cierre sin task[i].id"},
	}

	prompts := make(map[string]string)
	for _, tpl := range AgentTemplateCatalog() {
		prompts[tpl.Slug] = tpl.SystemPrompt
	}

	for _, c := range casos {
		t.Run(c.slug, func(t *testing.T) {
			prompt, ok := prompts[c.slug]
			require.True(t, ok, "%s debe estar en el catálogo", c.slug)

			bloque := bloqueOutputFormat(t, prompt)
			require.Contains(t, bloque, c.campo,
				"el <output_format> de %s no declara %s: %s", c.slug, c.campo, c.motivo)
		})
	}
}

// El ejemplo de la task verify obligatoria se copia literal, así que si no trae id
// el agente reproduce el mismo output que el validador rechaza.
func TestAgentTemplates_SddTasks_ElEjemploDeVerifyTraeID(t *testing.T) {
	var prompt string
	for _, tpl := range AgentTemplateCatalog() {
		if tpl.Slug == "sdd-tasks" {
			prompt = tpl.SystemPrompt
			break
		}
	}
	require.NotEmpty(t, prompt, "sdd-tasks debe estar en el catálogo")

	inicio := strings.Index(prompt, "<task_verify_obligatoria>")
	require.GreaterOrEqual(t, inicio, 0, "sdd-tasks debe declarar la task verify obligatoria")
	fin := strings.Index(prompt, "</task_verify_obligatoria>")
	require.Greater(t, fin, inicio, "el bloque task_verify_obligatoria debe estar cerrado")

	require.Contains(t, prompt[inicio:fin], `"id"`,
		"el ejemplo de la task verify se copia literal y el validador exige id en cada task")
}

// seeds.go skippea el seeder si applied_version >= Version(), así que sin bump los
// prompts corregidos no llegan a la BD y el viejo sigue gobernando en producción:
// el síntoma es indistinguible del éxito.
func TestAgentTemplatesSeedVersion_CubreElOutputFormat(t *testing.T) {
	require.GreaterOrEqual(t, agentTemplatesSeedVersion, 22,
		"corregir los <output_format> exige bump a 22; sin él el re-seed se skippea")
}

func bloqueOutputFormat(t *testing.T, prompt string) string {
	t.Helper()

	inicio := strings.Index(prompt, "<output_format>")
	require.GreaterOrEqual(t, inicio, 0, "el prompt debe declarar un bloque <output_format>")
	fin := strings.Index(prompt, "</output_format>")
	require.Greater(t, fin, inicio, "el bloque <output_format> debe estar cerrado")

	return prompt[inicio:fin]
}
