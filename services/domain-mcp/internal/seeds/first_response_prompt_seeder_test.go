package seeds

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFirstResponsePromptSeeder_ImplementsSeederInterface(t *testing.T) {
	var _ Seeder = (*FirstResponsePromptSeeder)(nil)
}

func TestFirstResponsePromptSeeder_Metadata(t *testing.T) {
	s := &FirstResponsePromptSeeder{}
	require.Equal(t, "first_response_prompt", s.Name())
	require.Equal(t, 6, s.Version())
	require.Equal(t, 63, s.Order())
	require.False(t, s.IsDevOnly())
}

func TestFirstResponsePromptSeeder_RunsAfterWizardFormulator(t *testing.T) {
	r := NewRegistry()
	r.Register(&FirstResponsePromptSeeder{})
	r.Register(&WizardFormulatorPromptSeeder{})
	sorted := r.Sorted()
	require.Equal(t, "wizard_formulator_prompt", sorted[0].Name())
	require.Equal(t, "first_response_prompt", sorted[1].Name())
}

func TestFirstResponsePromptSeeder_DefaultBodyNotEmpty(t *testing.T) {
	require.NotEmpty(t, strings.TrimSpace(DefaultFirstResponsePromptBody))
	// Las policies globales requieren una llamada aparte (no hay include_globals
	// para policies), asi que domain_policy_list sigue siendo obligatoria: es la
	// unica fuente del G de policies y el hook no la trae.
	require.Contains(t, DefaultFirstResponsePromptBody, "domain_policy_list")
	require.Contains(t, DefaultFirstResponsePromptBody, "domain_ticket_list")
	// DOMAINSERV-177: el hook SessionStart ya llama skill_list y policy_list
	// server-side e inyecta sus counts y slugs. Re-bajarlas en el turn 1 cuesta
	// ~40.000 chars de payload por datos que ya estan en contexto.
	require.Contains(t, DefaultFirstResponsePromptBody, "PASO 1 - Llamar 2 tools")
	require.Contains(t, DefaultFirstResponsePromptBody, "NO llamar domain_project_skill_list")
	// Estructura imperativa: pasos numerados + reglas duras contra omitir
	// skills/policies o parafrasear en prosa (feedback del usuario: sin
	// ambiguedad, la IA debe seguirlo a cabalidad).
	require.Contains(t, DefaultFirstResponsePromptBody, "PASO 1")
	require.Contains(t, DefaultFirstResponsePromptBody, "REGLAS DURAS")
	require.Contains(t, DefaultFirstResponsePromptBody, "PROHIBIDO omitir las globales")
}
