package mcpserver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-256: el scope por agente de DOMAINSERV-218 estaba construido y verde en 226 tests
// del paquete flow, y aun así NO existía en producción. La verificación con dos subagentes
// reales en paralelo (t15, 2026-08-07) lo midió: por el camino de domain_orchestrate el token
// salía SIN allowed_paths, así que el gate tomaba la rama "sin allowlist → sin restricción" y
// un agente editaba el territorio del otro sin ningún deny.
//
// La causa no estaba en el server sino en el CONTRATO de la tool: domain_orchestrate no
// declaraba allowed_paths, y el hook lo lee del tool_input (post-orchestrate.sh:79). Un
// parámetro no declarado se pierde de dos maneras y las dos son mudas — el cliente lo descarta
// antes de armar el tool_input, o lo propaga sin contrato de tipo y aterriza como STRING, con
// lo cual el isinstance(..., list) del hook lo tira igual.
//
// Estos son guards de contrato sobre el schema, no sobre la lógica: la lógica ya estaba bien.

func TestOrchestrate_DeclaraAllowedPaths_ParaQueElScopeLlegueAlHook(t *testing.T) {
	tool := toolOrchestrate()

	_, existe := tool.InputSchema.Properties["allowed_paths"]
	require.True(t, existe,
		"domain_orchestrate dejó de declarar allowed_paths: el hook lo lee del tool_input y un "+
			"parámetro no declarado se pierde en silencio, así que el token sale sin scope y el "+
			"gate degrada a FAIL-OPEN sin avisarle a nadie (DOMAINSERV-256)")
}

// El tipo importa tanto como la presencia, y por eso es un test aparte: el hook hace
// isinstance(ap, list) y cae al [] —"sin restricción de path"— ante cualquier otra cosa. Un
// allowed_paths declarado como string pasaría el test de arriba y seguiría sin aislar nada.
func TestOrchestrate_AllowedPathsEsArray_PorqueElHookDescartaCualquierOtroTipo(t *testing.T) {
	tool := toolOrchestrate()

	prop, existe := tool.InputSchema.Properties["allowed_paths"]
	require.True(t, existe, "domain_orchestrate dejó de declarar allowed_paths")

	m, ok := prop.(map[string]any)
	require.True(t, ok, "allowed_paths tiene que ser un objeto de schema")
	require.Equal(t, "array", m["type"],
		"allowed_paths dejó de ser array: el hook hace isinstance(ap, list) y ante cualquier otro "+
			"tipo cae al [], que significa 'el gate no restringe path'. Un string acá reproduce "+
			"exactamente el bug de DOMAINSERV-256 con el parámetro ya declarado")
}

// Guard sobre una afirmación factual, no sobre comportamiento. La descripción de project_id
// decía "flow_runs.project_id es NOT NULL" y es falso: la migración 000161 lo agregó como
// nullable y ninguna posterior lo cambió. El propio código de DOMAINSERV-218 lo sabe —
// scopeDelFlowRun lo escanea como *uuid.UUID y chequea nil. Una descripción que miente sobre
// el schema induce al error de asumir que el caso NULL no existe, y ese caso hoy DENIEGA el
// grant entero (handleFlowGrantToken:495-497).
func TestOrchestrate_ProjectID_NoAfirmaQueLaColumnaSeaNotNull(t *testing.T) {
	tool := toolOrchestrate()

	prop, existe := tool.InputSchema.Properties["project_id"]
	require.True(t, existe, "domain_orchestrate dejó de declarar project_id")

	m, ok := prop.(map[string]any)
	require.True(t, ok, "project_id tiene que ser un objeto de schema")
	desc, _ := m["description"].(string)
	require.NotEmpty(t, desc, "project_id dejó de tener descripción")

	require.NotContains(t, desc, "NOT NULL",
		"la descripción vuelve a afirmar que flow_runs.project_id es NOT NULL, y es falso: la "+
			"migración 000161 lo agregó nullable. Un flow_run con project_id NULL —los que crea "+
			"runner/flow/runner.go:191— hoy NO puede obtener token de edición")
}
