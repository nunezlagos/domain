// Tests de contrato del WizardFormulatorPromptSeeder sin tocar DB: identidad,
// versión, orden (después de analysis) y no-dev-only. El check-then-insert
// real se cubre en integración con testcontainers.

package seeds

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/wizardplan"
)

func TestWizardFormulatorPromptSeeder_ImplementsSeederInterface(t *testing.T) {
	var _ Seeder = (*WizardFormulatorPromptSeeder)(nil)
}

func TestWizardFormulatorPromptSeeder_Metadata(t *testing.T) {
	s := &WizardFormulatorPromptSeeder{}
	require.Equal(t, "wizard_formulator_prompt", s.Name())
	require.Equal(t, 1, s.Version())
	require.Equal(t, 62, s.Order())
	require.False(t, s.IsDevOnly())
}

// El seeder debe correr DESPUÉS del prompt de analysis (61).
func TestWizardFormulatorPromptSeeder_RunsAfterAnalysis(t *testing.T) {
	r := NewRegistry()
	r.Register(&WizardFormulatorPromptSeeder{})
	r.Register(&AnalysisPromptSeeder{})
	sorted := r.Sorted()
	require.Equal(t, "analysis_prompt", sorted[0].Name())
	require.Equal(t, "wizard_formulator_prompt", sorted[1].Name())
}

// El body que seedea debe ser el mismo const que usa el formulator como
// fallback — así editar el seed sin tocar el formulator no los desincroniza.
func TestWizardFormulatorPromptSeeder_BodyMatchesFormulatorConst(t *testing.T) {
	require.NotEmpty(t, strings.TrimSpace(wizardplan.DefaultFormulatorSystemPrompt))
	require.Contains(t, wizardplan.DefaultFormulatorSystemPrompt, "wizard interactivo")
}
