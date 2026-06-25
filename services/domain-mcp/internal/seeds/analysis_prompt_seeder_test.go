



package seeds

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	analysissvc "nunezlagos/domain/internal/service/orchestrator/analysis"
)

func TestAnalysisPromptSeeder_ImplementsSeederInterface(t *testing.T) {
	var _ Seeder = (*AnalysisPromptSeeder)(nil)
}

func TestAnalysisPromptSeeder_Metadata(t *testing.T) {
	s := &AnalysisPromptSeeder{}
	require.Equal(t, "analysis_prompt", s.Name())
	require.Equal(t, 1, s.Version())
	require.Equal(t, 61, s.Order())
	require.False(t, s.IsDevOnly())
}

// El seeder debe correr DESPUÉS del prompt de triage (60).
func TestAnalysisPromptSeeder_RunsAfterTriage(t *testing.T) {
	r := NewRegistry()
	r.Register(&AnalysisPromptSeeder{})
	r.Register(&TriagePromptSeeder{})
	sorted := r.Sorted()
	require.Equal(t, "triage_prompt", sorted[0].Name())
	require.Equal(t, "analysis_prompt", sorted[1].Name())
}

// El body que seedea debe ser el mismo const que usa el servicio como
// fallback — así editar el seed sin tocar el servicio no los desincroniza.
func TestAnalysisPromptSeeder_BodyMatchesServiceConst(t *testing.T) {
	require.NotEmpty(t, strings.TrimSpace(analysissvc.DefaultAnalysisSystemPrompt))
	require.Contains(t, analysissvc.DefaultAnalysisSystemPrompt, "analista técnico")
}
