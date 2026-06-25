package admin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestADR_HasQuantifiedTradeoffs assserta que el ADR del skill model
// (docs/rfc/0008-skill-model-simplification.md) contiene los números
// clave de los tradeoffs. Sin estos números, el ADR es opinión, no
// decisión basada en datos.
//
// Sabotaje documentado: escribir el ADR solo con las secciones
// "Contexto" y "Decisión" pero SIN mencionar los datos concretos
// de 35.4 ni las cifras de tradeoffs. Este test falla en ese caso.
func TestADR_HasQuantifiedTradeoffs(t *testing.T) {

	cwd, err := os.Getwd()
	require.NoError(t, err)
	repoRoot := cwd
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err == nil {
			break
		}
		repoRoot = filepath.Dir(repoRoot)
	}
	adrPath := filepath.Join(repoRoot, "docs", "rfc", "0008-skill-model-simplification.md")
	body, err := os.ReadFile(adrPath)
	require.NoError(t, err, "ADR not found at %s", adrPath)
	text := string(body)


	require.Contains(t, text, "245", "ADR debe mencionar 245 (agent_runs USADO)")
	require.Contains(t, text, "1023", "ADR debe mencionar 1023 (flow_runs USADO)")
	require.Contains(t, text, "NUNCA USADO", "ADR debe mencionar la categoría NUNCA USADO del skill_runner")
	require.Contains(t, text, "0", "ADR debe mencionar que el skill_runner tiene 0 ejecuciones")


	require.Contains(t, text, "500",
		"ADR debe cuantificar el código a remover (Opción A: ~500 líneas)")
	require.Contains(t, text, "2-4 semanas",
		"ADR debe cuantificar el trabajo de Opción B (2-4 semanas)")
	require.Contains(t, text, "94%",
		"ADR debe mencionar el % de skills que son TypePrompt (94%)")


	require.Contains(t, text, "Opción A",
		"ADR debe nombrar la opción ganadora (Opción A)")
	require.Contains(t, text, "accepted",
		"ADR debe marcar status (accepted, en línea con otros RFCs del repo)")
}

// TestADR_FollowsConvention assserta que el ADR tiene las secciones
// estándar de un RFC de domain:
//
//   - Contexto
//   - Datos (issue-specific)
//   - Opciones / Alternativas
//   - Decisión
//   - Consecuencias
//   - Implementación
//
// Si falta alguna, el ADR no sigue la convención del repo.
func TestADR_FollowsConvention(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	repoRoot := cwd
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err == nil {
			break
		}
		repoRoot = filepath.Dir(repoRoot)
	}
	adrPath := filepath.Join(repoRoot, "docs", "rfc", "0008-skill-model-simplification.md")
	body, err := os.ReadFile(adrPath)
	require.NoError(t, err)
	text := strings.ToLower(string(body))

	required := []string{
		"## contexto",
		"## datos",
		"## opciones consideradas",
		"## decisión",
		"## consecuencias",
		"## implementación",
	}
	for _, sec := range required {
		require.Contains(t, text, sec,
			"ADR debe tener la sección '%s'", sec)
	}
}

// TestADR_LinksToSourceData verifica que el ADR referencia los
// documentos de los que extrae los datos. Sin links, la decisión
// queda huérfana.
func TestADR_LinksToSourceData(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	repoRoot := cwd
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err == nil {
			break
		}
		repoRoot = filepath.Dir(repoRoot)
	}
	adrPath := filepath.Join(repoRoot, "docs", "rfc", "0008-skill-model-simplification.md")
	body, err := os.ReadFile(adrPath)
	require.NoError(t, err)
	text := string(body)

	require.Contains(t, text, "35.4", "ADR debe referenciar el issue 35.4")
	require.Contains(t, text, "docs/audit/2026-06-runners-coverage.md",
		"ADR debe linkear al reporte de auditoría")
	require.Contains(t, text, "REQ-05", "ADR debe referenciar el REQ-05 (skill system)")
}
