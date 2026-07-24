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

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() { os.Stderr = oldStderr }()

	_, ok := checkProjectRootGuard(empty)

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

	otherDir := t.TempDir()
	require.NoError(t, os.Chdir(otherDir))

	_, ok := checkProjectRootGuard(dir)
	require.True(t, ok, "checkProjectRootGuard with --src should pass for valid dir")

	cwd, _ := os.Getwd()
	require.True(t, strings.HasSuffix(cwd, filepath.Base(dir)),
		"cwd should be the --src path, got %s", cwd)
}

// TestCheckProjectRootGuard_SrcNotExists: --src apuntando a un path
// que no existe en absoluto -> ok=false con mensaje claro.
// issue-29.1 escenario 4: "--src /no/existe ... aborta con exit != 0"
func TestCheckProjectRootGuard_SrcNotExists(t *testing.T) {
	bogus := filepath.Join(t.TempDir(), "no-existe")

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() { os.Stderr = oldStderr }()

	_, ok := checkProjectRootGuard(bogus)

	_ = w.Close()
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	stderr := string(buf[:n])

	require.False(t, ok, "checkProjectRootGuard should fail for nonexistent --src")
	require.Contains(t, stderr, "project root check failed", "un --src inexistente aborta con el error de stat de IsProjectRoot")

}

// TestCheckProjectRootGuard_OnlyOneMarker: con solo .env.example
// (sin docker-compose.yml) -> ok=false y missing contiene ambos
// archivos que faltan en el reporte.
// issue-29.1 escenario 6: "solo uno de los dos archivos presentes
// → guard aborta (ambos deben estar presentes)".
func TestCheckProjectRootGuard_OnlyOneMarker(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env.example"), []byte("EX"), 0o600))

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() { os.Stderr = oldStderr }()

	_, ok := checkProjectRootGuard(dir)

	_ = w.Close()
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	stderr := string(buf[:n])

	require.False(t, ok, "checkProjectRootGuard should fail when solo uno de los markers existe")
	require.Contains(t, stderr, "docker-compose.yml",
		"mensaje debe mencionar docker-compose.yml como faltante")
}
