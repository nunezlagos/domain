package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func readDoc(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	return doc
}

func TestSetupClaudeCode_CreatesProjectConfig(t *testing.T) {
	dir := t.TempDir()
	path, err := SetupClaudeCode(dir, "/usr/local/bin/domain-mcp", "dk_test", "http://localhost:8000")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, ".mcp.json"), path)

	doc := readDoc(t, path)
	servers := doc["mcpServers"].(map[string]any)
	domain := servers["domain"].(map[string]any)
	require.Equal(t, "/usr/local/bin/domain-mcp", domain["command"])
	env := domain["env"].(map[string]any)
	require.Equal(t, "dk_test", env["DOMAIN_API_KEY"])


	_, err = SetupClaudeCode(dir, "/x", "", "")
	require.ErrorIs(t, err, ErrAlreadyConfigured)
}

func TestSetupClaudeCode_PreservesExistingServers(t *testing.T) {
	dir := t.TempDir()
	existing := `{"mcpServers": {"otro": {"command": "/bin/otro"}}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(existing), 0o600))

	path, err := SetupClaudeCode(dir, "/bin/domain-mcp", "", "")
	require.NoError(t, err)

	doc := readDoc(t, path)
	servers := doc["mcpServers"].(map[string]any)
	require.Contains(t, servers, "otro", "no debe pisar servers existentes")
	require.Contains(t, servers, "domain")


	matches, _ := filepath.Glob(filepath.Join(dir, ".mcp.json.bak.*"))
	require.Len(t, matches, 1)
}

func TestSetupOpenCode_InstallsAgentInstructions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()

	path, err := SetupOpenCode(dir, "/bin/domain-mcp", "dk_x", "")
	require.NoError(t, err)


	instr := home + "/.config/opencode/instructions/domain.md"
	data, err := os.ReadFile(instr)
	require.NoError(t, err, "instructions del agente deben instalarse")
	require.Contains(t, string(data), "domain tiene prioridad")
	require.Contains(t, string(data), "domain_mem_save")


	doc := readDoc(t, path)
	list, _ := doc["instructions"].([]any)
	require.Contains(t, list, any(instr))



	delete(doc, "instructions")
	out, _ := json.MarshalIndent(doc, "", "  ")
	require.NoError(t, os.WriteFile(path, out, 0o600))
	_, err = SetupOpenCode(dir, "/bin/domain-mcp", "dk_x", "")
	require.NoError(t, err, "upgrade debe aplicar instructions sin error")
	doc = readDoc(t, path)
	list, _ = doc["instructions"].([]any)
	require.Contains(t, list, any(instr), "upgrade agrega instructions al json existente")


	_, err = SetupOpenCode(dir, "/bin/domain-mcp", "dk_x", "")
	require.ErrorIs(t, err, ErrAlreadyConfigured)
}

func TestSetupOpenCode_Format(t *testing.T) {
	dir := t.TempDir()
	path, err := SetupOpenCode(dir, "/bin/domain-mcp", "dk_x", "")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, "opencode.json"), path)

	doc := readDoc(t, path)
	require.Equal(t, "https://opencode.ai/config.json", doc["$schema"])
	mcp := doc["mcp"].(map[string]any)
	domain := mcp["domain"].(map[string]any)
	require.Equal(t, "local", domain["type"])
	require.Equal(t, true, domain["enabled"])
	cmd := domain["command"].([]any)
	require.Equal(t, "/bin/domain-mcp", cmd[0])

	_, err = SetupOpenCode(dir, "/x", "", "")
	require.ErrorIs(t, err, ErrAlreadyConfigured)
}

func TestStatus_DetectsConfiguredAgents(t *testing.T) {
	dir := t.TempDir()
	_, err := SetupClaudeCode(dir, "/bin/domain-mcp", "", "")
	require.NoError(t, err)

	statuses := Status(dir)
	byAgent := map[Agent]AgentStatus{}
	for _, st := range statuses {
		byAgent[st.Agent] = st
	}
	require.True(t, byAgent[AgentClaudeCode].Configured)
	require.False(t, byAgent[AgentOpenCode].Configured)
}

func TestUninstall_RemovesOnlyDomain(t *testing.T) {
	dir := t.TempDir()
	existing := `{"mcpServers": {"otro": {"command": "/bin/otro"}}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(existing), 0o600))
	_, err := SetupClaudeCode(dir, "/bin/domain-mcp", "", "")
	require.NoError(t, err)

	path, removed, err := Uninstall(AgentClaudeCode, dir)
	require.NoError(t, err)
	require.True(t, removed)

	doc := readDoc(t, path)
	servers := doc["mcpServers"].(map[string]any)
	require.NotContains(t, servers, "domain")
	require.Contains(t, servers, "otro", "uninstall solo quita domain")


	_, removed, err = Uninstall(AgentClaudeCode, dir)
	require.NoError(t, err)
	require.False(t, removed)


	_, removed, err = Uninstall(AgentOpenCode, dir)
	require.NoError(t, err)
	require.False(t, removed)
}

// Sabotaje: config existente corrupto → setup NO sobrescribe.
func TestSabotage_CorruptConfig_NotOverwritten(t *testing.T) {
	dir := t.TempDir()
	corrupt := `{not json!!`
	cfgPath := filepath.Join(dir, ".mcp.json")
	require.NoError(t, os.WriteFile(cfgPath, []byte(corrupt), 0o600))

	_, err := SetupClaudeCode(dir, "/bin/domain-mcp", "", "")
	require.Error(t, err)

	raw, _ := os.ReadFile(cfgPath)
	require.Equal(t, corrupt, string(raw), "el config corrupto debe quedar intacto")
}
