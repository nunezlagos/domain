package mcpserver

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DOMAINSERV-154: un flow_run en 'pending' NUNCA arrancó — sus steps están
// todos pendientes y no hay trabajo a medias que retomar. Incluirlo entre los
// estados "activos" hacía que el hook UserPromptSubmit anunciara "flow SDD
// ACTIVO" en cada turno sobre runs que nunca se ejecutaron.
//
// Este es el test de sabotaje: si alguien vuelve a meter 'pending' en la
// lista, se pone rojo.
func TestFlowRunActiveStatuses_Pending_NoCuentaComoActivo(t *testing.T) {
	assert.NotContains(t, flowRunActiveStatuses, "pending",
		"un flow_run en pending nunca arrancó: anunciarlo como activo reporta trabajo inexistente en cada turno")
}

// El otro lado del invariante: un flow REALMENTE a medias tiene que seguir
// anunciándose. Esa señal es la que evita re-orquestar trabajo en curso, y
// perderla sería peor que el bug original.
func TestFlowRunActiveStatuses_EnCursoYPausados_SiCuentanComoActivos(t *testing.T) {
	for _, st := range []string{"running", "paused", "paused_awaiting_signal", "paused_awaiting_human"} {
		assert.Contains(t, flowRunActiveStatuses, st,
			"un flow en %q sí tiene trabajo a medias y debe seguir anunciándose", st)
	}
}

// La causa raíz de DOMAINSERV-154 no fue la query en sí, sino que HABÍA DOS
// respondiendo la misma pregunta con criterios distintos: el bootstrap excluía
// 'pending' y activeFlowRunID lo incluía. Este test ancla la unificación —
// ninguna de las dos puede volver a hardcodear su propia lista.
func TestFlowRunActiveStatuses_NingunaQueryHardcodeaSuPropiaLista(t *testing.T) {
	for _, f := range []string{"captured_prompt_tools.go", "session_bootstrap_tools.go"} {
		src, err := os.ReadFile(f)
		require.NoError(t, err, "no se pudo leer %s", f)

		assert.NotContains(t, string(src), "'pending','running'",
			"%s volvió a hardcodear la lista de estados en vez de usar flowRunActiveStatuses", f)
		assert.Contains(t, string(src), "flowRunActiveStatuses",
			"%s debe derivar los estados activos de la constante compartida", f)
	}
}

// El orden de la lista no importa para la semántica, pero sí que no queden
// duplicados: un duplicado sería señal de que alguien agregó un estado sin
// mirar los que ya estaban.
func TestFlowRunActiveStatuses_SinDuplicados(t *testing.T) {
	visto := map[string]bool{}
	for _, st := range flowRunActiveStatuses {
		assert.False(t, visto[st], "estado %q duplicado en flowRunActiveStatuses", st)
		visto[st] = true
		assert.Equal(t, strings.TrimSpace(st), st, "estado %q tiene espacios alrededor", st)
	}
}
