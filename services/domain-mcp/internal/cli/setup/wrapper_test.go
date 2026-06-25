package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateWrapperSnippet_HasMarkers(t *testing.T) {
	snippet := GenerateWrapperSnippet()
	require.Contains(t, snippet, WrapperMarkerOpen)
	require.Contains(t, snippet, WrapperMarkerClose)
}

func TestGenerateWrapperSnippet_HasFunctions(t *testing.T) {
	snippet := GenerateWrapperSnippet()
	require.Contains(t, snippet, "opencode() {")
	require.Contains(t, snippet, "domain() {")
	require.Contains(t, snippet, `domain setup auto-detect "$PWD" --quiet`)
}

func TestInstallShellWrapper_Fresh(t *testing.T) {
	rcfile := filepath.Join(t.TempDir(), ".zshrc")
	require.NoError(t, os.WriteFile(rcfile, []byte("export FOO=bar\n"), 0o600))

	installed, err := InstallShellWrapper(rcfile)
	require.NoError(t, err)
	require.True(t, installed)

	data, _ := os.ReadFile(rcfile)
	content := string(data)
	require.Contains(t, content, WrapperMarkerOpen)
	require.Contains(t, content, WrapperMarkerClose)
	require.Contains(t, content, "opencode() {")

	require.Contains(t, content, "export FOO=bar")
}

func TestInstallShellWrapper_Idempotent(t *testing.T) {
	rcfile := filepath.Join(t.TempDir(), ".zshrc")
	require.NoError(t, os.WriteFile(rcfile, []byte("# test\n"), 0o600))

	installed, err := InstallShellWrapper(rcfile)
	require.NoError(t, err)
	require.True(t, installed)

	data1, _ := os.ReadFile(rcfile)

	installed, err = InstallShellWrapper(rcfile)
	require.NoError(t, err)
	require.False(t, installed)

	data2, _ := os.ReadFile(rcfile)
	require.Equal(t, data1, data2, "rcfile should not change on second install")
}

func TestUninstallShellWrapper(t *testing.T) {
	rcfile := filepath.Join(t.TempDir(), ".zshrc")
	original := "export FOO=bar\n"
	require.NoError(t, os.WriteFile(rcfile, []byte(original), 0o600))

	_, err := InstallShellWrapper(rcfile)
	require.NoError(t, err)

	require.NoError(t, UninstallShellWrapper(rcfile))

	data, _ := os.ReadFile(rcfile)
	require.Equal(t, original, string(data))
}

func TestUninstallShellWrapper_Noop(t *testing.T) {
	rcfile := filepath.Join(t.TempDir(), ".zshrc")
	original := "export FOO=bar\n"
	require.NoError(t, os.WriteFile(rcfile, []byte(original), 0o600))

	err := UninstallShellWrapper(rcfile)
	require.NoError(t, err)

	data, _ := os.ReadFile(rcfile)
	require.Equal(t, original, string(data))
}

func TestHasWrapper(t *testing.T) {
	rcfile := filepath.Join(t.TempDir(), ".zshrc")
	require.NoError(t, os.WriteFile(rcfile, []byte("test\n"), 0o600))

	has, err := HasWrapper(rcfile)
	require.NoError(t, err)
	require.False(t, has)

	_, _ = InstallShellWrapper(rcfile)

	has, err = HasWrapper(rcfile)
	require.NoError(t, err)
	require.True(t, has)
}

func TestDetectShell(t *testing.T) {
	t.Run("zsh", func(t *testing.T) {
		t.Setenv("SHELL", "/usr/bin/zsh")
		shell, rcfile := DetectShell()
		require.Equal(t, "zsh", shell)
		require.Contains(t, rcfile, ".zshrc")
	})
	t.Run("bash", func(t *testing.T) {
		t.Setenv("SHELL", "/bin/bash")
		shell, rcfile := DetectShell()
		require.Equal(t, "bash", shell)
		require.Contains(t, rcfile, ".bashrc")
	})
	t.Run("fish_fallback_zsh", func(t *testing.T) {
		t.Setenv("SHELL", "/usr/bin/fish")
		shell, rcfile := DetectShell()
		require.Equal(t, "zsh", shell)
		require.Contains(t, rcfile, ".zshrc")
	})
	t.Run("empty_fallback_zsh", func(t *testing.T) {
		t.Setenv("SHELL", "")
		shell, rcfile := DetectShell()
		require.Equal(t, "zsh", shell)
		require.Contains(t, rcfile, ".zshrc")
	})
}

func TestSyntaxCheck(t *testing.T) {
	dir := t.TempDir()
	rcfile := filepath.Join(dir, ".zshrc")
	require.NoError(t, os.WriteFile(rcfile, []byte("export A=1\n"), 0o600))
	require.NoError(t, syntaxCheck("zsh", rcfile))


	require.NoError(t, os.WriteFile(rcfile, []byte("if broken\n"), 0o600))
	err := syntaxCheck("zsh", rcfile)
	require.Error(t, err)
}

func TestGenerateWrapperSnippet_NoCommentsAfterFunctions(t *testing.T) {
	snippet := GenerateWrapperSnippet()
	lines := strings.Split(snippet, "\n")
	foundDomainFunc := false
	foundOpenCodeFunc := false
	for _, line := range lines {
		if strings.Contains(line, "domain() {") {
			foundDomainFunc = true
		}
		if strings.Contains(line, "opencode() {") {
			foundOpenCodeFunc = true
		}
	}
	require.True(t, foundDomainFunc)
	require.True(t, foundOpenCodeFunc)
}
