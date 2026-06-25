




package seeds

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCatalogSeeders_ImplementSeederInterface(t *testing.T) {
	var _ Seeder = (*SkillsCatalogSeeder)(nil)
	var _ Seeder = (*AgentTemplatesCatalogSeeder)(nil)
	var _ Seeder = (*FlowsCatalogSeeder)(nil)
}

func TestCatalogSeeders_Metadata(t *testing.T) {
	cases := []struct {
		seeder      Seeder
		wantName    string
		wantVersion int
		wantOrder   int
	}{
		{&SkillsCatalogSeeder{}, "skills", skillsSeedVersion, 50},
		{&AgentTemplatesCatalogSeeder{}, "agent_templates", agentTemplatesSeedVersion, 51},
		{&FlowsCatalogSeeder{}, "flows", flowsSeedVersion, 52},
	}
	for _, c := range cases {
		t.Run(c.wantName, func(t *testing.T) {
			require.Equal(t, c.wantName, c.seeder.Name())
			require.Equal(t, c.wantVersion, c.seeder.Version())
			require.Equal(t, c.wantOrder, c.seeder.Order())
			require.False(t, c.seeder.IsDevOnly())
		})
	}
}

// Los catálogos globales deben ordenarse DESPUÉS de los seeders base
// (platform_policies=30, project_templates=35, mcp_providers=40).
func TestCatalogSeeders_RunAfterBaseSeeders(t *testing.T) {
	r := NewRegistry()
	r.Register(&PlatformPoliciesSeeder{})
	r.Register(&ProjectTemplatesSeeder{})
	r.Register(&MCPProvidersSeeder{})
	r.Register(&SkillsCatalogSeeder{})
	r.Register(&AgentTemplatesCatalogSeeder{})
	r.Register(&FlowsCatalogSeeder{})

	sorted := r.Sorted()
	require.Equal(t, []string{
		"platform_policies",
		"project_templates",
		"mcp_providers",
		"skills",
		"agent_templates",
		"flows",
	}, names(sorted))
}

func names(ss []Seeder) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = s.Name()
	}
	return out
}
