package claudehook

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHasDomainHook_Fresh(t *testing.T) {
	doc := map[string]any{}
	require.False(t, HasDomainHook(doc))
}

func TestHasDomainHook_AlreadyThere(t *testing.T) {
	doc := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{"type": "command", "command": `domain setup auto-detect "$PWD" --quiet`},
			},
		},
	}
	require.True(t, HasDomainHook(doc))
}

func TestHasDomainHook_PartialMatch(t *testing.T) {
	doc := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{"type": "command", "command": `echo "domain setup auto-detect"`},
			},
		},
	}
	require.False(t, HasDomainHook(doc))
}

func TestHasDomainHook_WrongType(t *testing.T) {
	doc := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{"type": "bash", "command": `domain setup auto-detect "$PWD" --quiet`},
			},
		},
	}
	require.False(t, HasDomainHook(doc))
}

func TestAddDomainHook_PreservesOther(t *testing.T) {
	doc := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{"type": "command", "command": `echo "hi"`},
			},
		},
	}
	result := AddDomainHook(doc)
	hooks, _ := result["hooks"].(map[string]any)
	ss, _ := hooks["SessionStart"].([]any)
	require.Len(t, ss, 2)
	first := ss[0].(map[string]any)
	require.Equal(t, `echo "hi"`, first["command"])
	second := ss[1].(map[string]any)
	require.Contains(t, second["command"].(string), "domain setup auto-detect")
}

func TestAddDomainHook_PreservesOtherKeys(t *testing.T) {
	doc := map[string]any{
		"theme": "dark",
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{"type": "command", "command": "echo hi"},
			},
		},
	}
	result := AddDomainHook(doc)
	require.Equal(t, "dark", result["theme"])
}

func TestAddDomainHook_NilHooks(t *testing.T) {
	doc := map[string]any{}
	result := AddDomainHook(doc)
	hooks, _ := result["hooks"].(map[string]any)
	ss, _ := hooks["SessionStart"].([]any)
	require.Len(t, ss, 1)
	hook := ss[0].(map[string]any)
	require.Contains(t, hook["command"].(string), "domain setup auto-detect")
}

func TestReadSettings_NotExists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	doc, raw, err := ReadSettings()
	require.NoError(t, err)
	require.Nil(t, raw)
	require.NotNil(t, doc)
	require.Empty(t, doc)
}

func TestReadSettings_Exists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	settingsDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(settingsDir, 0o755))
	settingsPath := filepath.Join(settingsDir, "settings.json")
	content := `{"theme": "light", "hooks": {"SessionStart": [{"type": "command", "command": "echo test"}]}}`
	require.NoError(t, os.WriteFile(settingsPath, []byte(content), 0o600))

	doc, raw, err := ReadSettings()
	require.NoError(t, err)
	require.NotNil(t, raw)
	require.Equal(t, "light", doc["theme"])
}

func TestReadSettings_Malformed(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	settingsDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(settingsDir, 0o755))
	settingsPath := filepath.Join(settingsDir, "settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte(`{not json}`), 0o600))

	doc, raw, err := ReadSettings()
	require.NoError(t, err)
	require.NotNil(t, raw)
	require.NotNil(t, doc)
	require.Empty(t, doc)
}

func TestInstallClaudeHook_Fresh(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	action, err := InstallClaudeHook(false, true)
	require.NoError(t, err)
	require.Equal(t, "installed", action)

	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	_, err = os.Stat(settingsPath)
	require.NoError(t, err)

	doc, _, err := ReadSettings()
	require.NoError(t, err)
	require.True(t, HasDomainHook(doc))
}

func TestInstallClaudeHook_AlreadyInstalled(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	action, err := InstallClaudeHook(false, true)
	require.NoError(t, err)
	require.Equal(t, "installed", action)

	action, err = InstallClaudeHook(false, true)
	require.NoError(t, err)
	require.Equal(t, "already_installed", action)
}

func TestInstallClaudeHook_NonInteractiveSkip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	action, err := InstallClaudeHook(true, false)
	require.NoError(t, err)
	require.Equal(t, "skipped", action)

	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	_, err = os.Stat(settingsPath)
	require.True(t, os.IsNotExist(err))
}

func TestSettingsPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	expected := filepath.Join(dir, ".claude", "settings.json")
	require.Equal(t, expected, SettingsPath())
}

func TestInstallClaudeHook_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	_, err := InstallClaudeHook(false, true)
	require.NoError(t, err)

	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	info, err := os.Stat(settingsPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
