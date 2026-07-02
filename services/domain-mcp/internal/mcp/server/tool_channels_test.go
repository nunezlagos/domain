package mcpserver

import (
	"testing"
)

// REQ-54 issue-54.6: la invariante central de la matriz de cobertura.
// Toda tool del registry vivo DEBE tener canal asignado. Una tool nueva sin
// clasificar rompe CI acá — así "cubrir el 100% de las tools" queda congelado
// como hecho verificable, no aspiración.
func TestAllToolsHaveChannel(t *testing.T) {
	t.Parallel()
	tools := Tools(Deps{})
	if len(tools) == 0 {
		t.Fatal("registry vacío: Tools(Deps{}) no devolvió tools")
	}
	var missing []string
	for _, st := range tools {
		name := st.Tool.Name
		if _, ok := toolChannel[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("tools SIN canal en la matriz (clasificalas en tool_channels.go): %v", missing)
	}
}

// La inversa: entradas de la matriz que ya no existen en el registry son
// deuda de limpieza (tool renombrada/eliminada).
func TestMatrixHasNoOrphanEntries(t *testing.T) {
	t.Parallel()
	registry := map[string]bool{}
	for _, st := range Tools(Deps{}) {
		registry[st.Tool.Name] = true
	}
	var orphans []string
	for name := range toolChannel {
		if !registry[name] {
			orphans = append(orphans, name)
		}
	}
	if len(orphans) > 0 {
		t.Fatalf("entradas de la matriz sin tool en el registry (limpiar tool_channels.go): %v", orphans)
	}
}

// Sanity de la distribución: los canales automáticos (hook/first-response/
// contract/prep) no pueden quedar vacíos — si esto falla, alguien desarmó
// la cobertura automática.
func TestChannelDistribution_AutomaticChannelsNonEmpty(t *testing.T) {
	t.Parallel()
	count := map[ToolChannel]int{}
	for _, ch := range toolChannel {
		count[ch]++
	}
	for _, ch := range []ToolChannel{ChannelHook, ChannelFirstResponse, ChannelPhaseContract, ChannelPolicyTriggered} {
		if count[ch] == 0 {
			t.Errorf("canal %s sin tools asignadas", ch)
		}
	}
	t.Logf("distribución: %v (total %d)", count, len(toolChannel))
}
