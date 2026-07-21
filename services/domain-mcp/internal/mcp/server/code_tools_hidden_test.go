package mcpserver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// codeToolNames son las tools del code graph (retiradas 2026-07-07): deben
// quedar fuera del manifest por default para no contaminar el tool-search.
var codeToolNames = []string{
	"domain_code_build", "domain_code_upload", "domain_code_explore",
	"domain_code_path", "domain_code_graph", "domain_code_observations",
	"domain_mem_link_code", "domain_mem_code_links",
}

func manifestToolNames(deps Deps) map[string]bool {
	set := map[string]bool{}
	for _, st := range Tools(deps) {
		set[st.Tool.Name] = true
	}
	return set
}

func TestCodeTools_HiddenFromManifestByDefault(t *testing.T) {
	names := manifestToolNames(Deps{})
	for _, n := range codeToolNames {
		require.False(t, names[n], "%s NO debe estar en el manifest por default", n)
	}
	// control: una tool viva sí debe estar
	require.True(t, names["domain_mem_save"])
}

func TestCodeTools_ExposedWhenEnvEnabled(t *testing.T) {
	t.Setenv("DOMAIN_EXPOSE_CODE_TOOLS", "true")
	names := manifestToolNames(Deps{})
	for _, n := range codeToolNames {
		require.True(t, names[n], "%s debe re-exponerse con DOMAIN_EXPOSE_CODE_TOOLS=true", n)
	}
}
