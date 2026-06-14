package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCheckProjectRootGuard_OKInRepo: si cwd es el root del repo,
// retorna ok=true sin requerir --src.
//
// Localizamos el root subiendo desde el archivo de test hasta
// encontrar `.env.example` (más robusto que asumir el cwd).
func TestCheckProjectRootGuard_OKInRepo(t *testing.T) {
	repoRoot := findRepoRoot(t)
	require.NoError(t, os.Chdir(repoRoot), "chdir to repo root should succeed")

	_, ok := checkProjectRootGuard("")
	require.True(t, ok, "checkProjectRootGuard should pass at the repo root")
}

// findRepoRoot walks up from the test file's directory until it
// finds a directory containing `.env.example`. Retorna ese path.
// Falla el test si no lo encuentra (asume que el test corre
// dentro del repo domain).
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, ".env.example")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (.env.example) walking up from test dir")
		}
		dir = parent
	}
	t.Fatal("walked up 10 levels without finding .env.example")
	return ""
}

// TestCheckProjectRootGuard_FailsOutsideRepo: con --src apuntando
// a tempdir vacío, retorna ok=false.
func TestCheckProjectRootGuard_FailsOutsideRepo(t *testing.T) {
	empty := t.TempDir() // sin .env.example ni docker-compose.yml

	// Capturar stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() { os.Stderr = oldStderr }()

	_, ok := checkProjectRootGuard(empty)

	// Cerrar el write end para que el read retorne
	_ = w.Close()
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	stderr := string(buf[:n])

	require.False(t, ok, "checkProjectRootGuard should fail for empty dir")
	require.Contains(t, stderr, "no estás en el root del repo domain")
	require.Contains(t, stderr, ".env.example")
	require.Contains(t, stderr, "docker-compose.yml")
}

// TestCheckProjectRootGuard_SrcOverrideOK: --src apuntando a
// tempdir con ambos archivos -> ok=true Y chdir al src.
func TestCheckProjectRootGuard_SrcOverrideOK(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env.example"), []byte("EX"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("CO"), 0o600))

	originalCwd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalCwd) }()

	// chdir a un dir random antes, para verificar que --src cambia
	otherDir := t.TempDir()
	require.NoError(t, os.Chdir(otherDir))

	_, ok := checkProjectRootGuard(dir)
	require.True(t, ok, "checkProjectRootGuard with --src should pass for valid dir")

	// Después del guard, el cwd efectivo debe ser --src.
	cwd, _ := os.Getwd()
	require.True(t, strings.HasSuffix(cwd, filepath.Base(dir)),
		"cwd should be the --src path, got %s", cwd)
}
