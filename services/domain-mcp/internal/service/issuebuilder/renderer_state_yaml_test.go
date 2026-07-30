package issuebuilder

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-210: el state.yaml del preview salía con created: 2026-06-09 en un draft
// creado el 2026-07-30, porque la fecha estaba escrita a mano en el renderer. Es el
// archivo que se versiona en el repo tal cual sale del preview.
func TestRenderStateYaml_UsaLaFechaDeCreacionDelDraft(t *testing.T) {
	creado := time.Date(2026, 7, 30, 21, 14, 0, 0, time.UTC)

	yaml := renderStateYaml(creado)

	require.Contains(t, yaml, "created: 2026-07-30",
		"el state.yaml debe llevar la fecha de creación del draft")
	require.NotContains(t, yaml, "2026-06-09",
		"la fecha hardcodeada no puede sobrevivir en ninguna forma")
}

// El preview completo es lo que el usuario copia al repo: si el renderer toma la fecha
// pero el preview no se la pasa, el defecto sigue vivo donde importa.
func TestRenderFeaturePreview_StateYaml_LlevaLaFechaDelDraft(t *testing.T) {
	d := &Draft{
		InitialIdea: "acotar el cuerpo de los tools de listado",
		CreatedAt:   time.Date(2026, 7, 30, 21, 14, 0, 0, time.UTC),
	}
	answers := map[string]any{
		"slug":        "fix-derrame-de-cuerpo",
		"req_parent":  "REQ-54-tool-contract",
		"audience":    "dx-engineer",
		"effort":      "M",
		"priority":    "alta",
		"change_type": "fix",
		"goal":        "que los listados no derramen el cuerpo",
		"summary":     "proyección por tool",
	}

	pv, err := renderFeaturePreview(d, answers)
	require.NoError(t, err)

	yaml, ok := pv.Files["state.yaml"]
	require.True(t, ok, "el preview debe incluir state.yaml")
	require.Contains(t, yaml, "created: 2026-07-30")
}

// Un draft sin CreatedAt no puede escribir "created: 0001-01-01" en un archivo que se
// versiona: el zero value se degrada a la fecha de hoy.
func TestRenderStateYaml_DraftSinFecha_NoEscribeElZeroValue(t *testing.T) {
	yaml := renderStateYaml(time.Time{})

	require.NotContains(t, yaml, "0001-01-01",
		"el zero value de time.Time no puede llegar al archivo versionado")

	linea := lineaCreated(t, yaml)
	require.Equal(t, "created: "+time.Now().UTC().Format("2006-01-02"), linea)
}

func lineaCreated(t *testing.T, yaml string) string {
	t.Helper()

	for _, l := range strings.Split(yaml, "\n") {
		if strings.HasPrefix(l, "created:") {
			return l
		}
	}
	t.Fatal("el state.yaml debe declarar created:")
	return ""
}
