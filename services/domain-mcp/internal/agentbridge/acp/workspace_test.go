package acp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestWorkspace(t *testing.T) *Workspace {
	t.Helper()
	w, err := NewWorkspace()
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Cleanup() })
	return w
}

func TestWorkspace_Resolve_InsideRoot_Ok(t *testing.T) {
	w := newTestWorkspace(t)
	got, err := w.resolve("sub/file.txt")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(w.Root(), "sub/file.txt"), got)
}

func TestWorkspace_Resolve_Traversal_Rejected(t *testing.T) {
	w := newTestWorkspace(t)

	_, err := w.resolve("../etc/passwd")
	require.Error(t, err, "traversal relativo debe rechazarse")

	_, err = w.resolve("/etc/passwd")
	require.Error(t, err, "ruta absoluta fuera del root debe rechazarse")
}

func TestWorkspace_Resolve_SymlinkEscape_Rejected(t *testing.T) {
	w := newTestWorkspace(t)

	outside, err := os.MkdirTemp("", "acp-outside-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(outside) })
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret"), []byte("x"), 0o600))

	link := filepath.Join(w.Root(), "escape")
	require.NoError(t, os.Symlink(outside, link))

	_, err = w.resolve("escape/secret")
	require.Error(t, err, "symlink que apunta afuera del root debe rechazarse")
}
