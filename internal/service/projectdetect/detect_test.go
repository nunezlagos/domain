// F2: projectdetect — tests unitarios puros (sin DB, sin testcontainers).

package projectdetect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetect_FindsGitRemote(t *testing.T) {
	tmp := t.TempDir()
	gitDir := filepath.Join(tmp, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "config"),
		[]byte(`[remote "origin"]
	url = git@github.com:acme/widgets.git
`), 0o600))

	m, err := Detect(tmp)
	require.NoError(t, err)
	require.Equal(t, "acme/widgets", m.GitRemote)
	require.Equal(t, "git@github.com:acme/widgets.git", m.GitRemoteURL)
}

func TestDetect_HttpsRemote(t *testing.T) {
	tmp := t.TempDir()
	gitDir := filepath.Join(tmp, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "config"),
		[]byte(`[remote "origin"]
	url = https://github.com/acme/widgets.git
`), 0o600))

	m, err := Detect(tmp)
	require.NoError(t, err)
	require.Equal(t, "acme/widgets", m.GitRemote)
}

func TestDetect_FindsCurrentBranch(t *testing.T) {
	tmp := t.TempDir()
	gitDir := filepath.Join(tmp, ".git")
	require.NoError(t, os.MkdirAll(filepath.Join(gitDir, "refs", "heads"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "HEAD"),
		[]byte("ref: refs/heads/feature/foo\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "config"), []byte{}, 0o600))

	m, err := Detect(tmp)
	require.NoError(t, err)
	require.Equal(t, "feature/foo", m.CurrentBranch)
}

func TestDetect_DetectsGoStack(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0o644))

	m, err := Detect(tmp)
	require.NoError(t, err)
	require.Equal(t, StackGo, m.Stack)
}

func TestDetect_DetectsPHPStack(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "composer.json"), []byte(`{"name": "acme/test"}`), 0o644))

	m, err := Detect(tmp)
	require.NoError(t, err)
	require.Equal(t, StackPHP, m.Stack)
}

func TestDetect_DetectsNodeStack(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "package.json"), []byte(`{"name": "acme"}`), 0o644))

	m, err := Detect(tmp)
	require.NoError(t, err)
	require.Equal(t, StackNode, m.Stack)
}

func TestDetect_DetectsPythonStack(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "pyproject.toml"), []byte("[project]\nname = 'x'\n"), 0o644))

	m, err := Detect(tmp)
	require.NoError(t, err)
	require.Equal(t, StackPython, m.Stack)
}

func TestDetect_FindsManifestYAML(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, ".domain"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, ".domain", "manifest.yaml"),
		[]byte("project_slug: my-app\n"), 0o644))

	m, err := Detect(tmp)
	require.NoError(t, err)
	require.True(t, m.HasManifest)
	require.Equal(t, filepath.Join(tmp, ".domain", "manifest.yaml"), m.ManifestPath)
}

func TestDetect_ProjectSlugFromRepo(t *testing.T) {
	tmp := t.TempDir()
	gitDir := filepath.Join(tmp, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "config"),
		[]byte(`[remote "origin"]
	url = git@github.com:acme/widgets.git
`), 0o600))

	m, err := Detect(tmp)
	require.NoError(t, err)
	require.Equal(t, "acme-widgets", m.ProjectSlug)
}

func TestDetect_NoGitNoManifestNoStack_Errors(t *testing.T) {
	tmp := t.TempDir()
	_, err := Detect(tmp)
	require.ErrorIs(t, err, ErrNotInProject)
}

func TestDetect_SlugifyFromPath(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module x\n"), 0o644))

	m, err := Detect(tmp)
	require.NoError(t, err)
	require.NotEmpty(t, m.ProjectSlug)
	require.NotContains(t, m.ProjectSlug, " ")
	require.NotContains(t, m.ProjectSlug, "/")
}

func TestDetect_FindsGitRoot_GoingUp(t *testing.T) {
	tmp := t.TempDir()
	gitDir := filepath.Join(tmp, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "config"),
		[]byte(`[remote "origin"]
	url = git@github.com:acme/widgets.git
`), 0o600))
	subdir := filepath.Join(tmp, "src", "pkg")
	require.NoError(t, os.MkdirAll(subdir, 0o755))

	m, err := Detect(subdir)
	require.NoError(t, err)
	require.Equal(t, "acme/widgets", m.GitRemote)
}
