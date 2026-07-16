package seeds

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVoseoGuard_AssertNeutralSpanish_VoseoInput_ReturnsError(t *testing.T) {
	cases := []string{
		"Detectá las áreas del codebase",
		"corré el test antes de commitear",
		"Si dudás, dejá el campo vacío",
		"Reportá los resultados y persistí el estado",
	}
	for _, c := range cases {
		if err := assertNeutralSpanish("test", c); err == nil {
			t.Errorf("esperaba voseo detectado en %q", c)
		}
	}
}

func TestVoseoGuard_AssertNeutralSpanish_NeutralInput_ReturnsNil(t *testing.T) {
	ok := "Detecta las áreas, corre el test, si dudas no incluyas nada. " +
		"El deploy será hoy; ya agregué y edité la línea; el país está acá y aquí, así que listo."
	if err := assertNeutralSpanish("test", ok); err != nil {
		t.Errorf("falso positivo: %v", err)
	}
}

// TestVoseoGuard_SourceFiles_NoVoseo recorre los seeders de texto y los prompts de
// las phases del orchestrator y falla el build ante voseo. Excluye el propio guard
// (contiene voseo como dato) y los _test.go.
func TestVoseoGuard_SourceFiles_NoVoseo(t *testing.T) {
	dirs := []string{".", "../service/orchestrator/phases"}
	for _, dir := range dirs {
		matches, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range matches {
			base := filepath.Base(f)
			if strings.HasSuffix(base, "_test.go") || base == "voseo_guard.go" {
				continue
			}
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if err := assertNeutralSpanish(f, string(b)); err != nil {
				t.Error(err)
			}
		}
	}
}
