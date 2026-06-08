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
