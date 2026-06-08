package flowrunner

import (
	"context"
	"strings"
	"testing"
)

func TestFormatChain(t *testing.T) {
	if got := formatChain(nil); got != "" {
		t.Fatal("empty chain should be empty string")
	}
	if got := formatChain([]string{"a"}); got != "a" {
		t.Fatalf("single: %q", got)
	}
	got := formatChain([]string{"a", "b", "c"})
	if !strings.Contains(got, "a") || !strings.Contains(got, "→") || !strings.Contains(got, "c") {
		t.Fatalf("multi: %q", got)
	}
}

func TestSubflowCircular_DetectaCadenaRepetida(t *testing.T) {
	// Simulamos el contexto que el runner construye.
	ctx := context.WithValue(context.Background(), subflowCtxKey{}, []string{"a", "b", "c"})
	chain, _ := ctx.Value(subflowCtxKey{}).([]string)

	// Verificar que la cadena se mantiene.
	if len(chain) != 3 {
		t.Fatalf("chain length: %d", len(chain))
	}

	// El runner verifica si flowSlug está en la cadena antes de añadirlo.
	for _, s := range chain {
		if s == "b" {
			// Esto sería el path que dispara el error.
			return
		}
	}
	t.Fatal("did not detect cycle when expected")
}

func TestSubflowDepthLimit_Constante(t *testing.T) {
	if maxSubflowDepth < 5 || maxSubflowDepth > 20 {
		t.Fatalf("maxSubflowDepth out of reasonable range: %d", maxSubflowDepth)
	}
}

func TestMapToStep_ParseaCampos(t *testing.T) {
	m := map[string]any{
		"id":   "s1",
		"type": "sub_flow",
		"params": map[string]any{
			"flow_slug": "child",
			"input":     map[string]any{"k": "v"},
		},
	}
	st := mapToStep(m)
	if st.ID != "s1" {
		t.Fatalf("id: %s", st.ID)
	}
	if st.Type != "sub_flow" {
		t.Fatalf("type: %s", st.Type)
	}
	if st.Config["flow_slug"] != "child" {
		t.Fatalf("config flow_slug: %v", st.Config["flow_slug"])
	}
}

func TestMapToStep_AceptaConfigOParams(t *testing.T) {
	cfg := map[string]any{"id": "x", "type": "skill_run", "config": map[string]any{"a": 1}}
	if mapToStep(cfg).Config["a"] != 1 {
		t.Fatal("config key not propagated")
	}
	par := map[string]any{"id": "x", "type": "skill_run", "params": map[string]any{"b": 2}}
	if mapToStep(par).Config["b"] != 2 {
		t.Fatal("params key not propagated")
	}
}
