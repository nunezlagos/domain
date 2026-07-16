



package seeds

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/service/promptrouter"
)

func TestTriagePromptSeeder_ImplementsSeederInterface(t *testing.T) {
	var _ Seeder = (*TriagePromptSeeder)(nil)
}

func TestTriagePromptSeeder_Metadata(t *testing.T) {
	s := &TriagePromptSeeder{}
	require.Equal(t, "triage_prompt", s.Name())
	require.Equal(t, 2, s.Version())
	require.Equal(t, 60, s.Order())
	require.False(t, s.IsDevOnly())
}

// El seeder debe correr DESPUÉS de los catálogos (flows=52).
func TestTriagePromptSeeder_RunsAfterCatalogs(t *testing.T) {
	r := NewRegistry()
	r.Register(&FlowsCatalogSeeder{})
	r.Register(&TriagePromptSeeder{})
	sorted := r.Sorted()
	require.Equal(t, "flows", sorted[0].Name())
	require.Equal(t, "triage_prompt", sorted[1].Name())
}

// El body que seedea debe ser el mismo const que usa el classifier como
// fallback — así editar el seed sin tocar el classifier no los desincroniza.
func TestTriagePromptSeeder_BodyMatchesClassifierConst(t *testing.T) {
	require.NotEmpty(t, strings.TrimSpace(promptrouter.DefaultTriageSystemPrompt))
	require.Contains(t, promptrouter.DefaultTriageSystemPrompt, "clasificador de prompts")
}
