package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRunInstall_AbortsOutsideRepo es el e2e sabotaje-resistente:
// corre `runInstall` desde un tempdir sin .env.example y verifica
// que aborta con exit 1 + mensaje claro. Si el guard de cwd
// está deshabilitado (sabotaje), este test DEBE FALLAR.
func TestRunInstall_AbortsOutsideRepo(t *testing.T) {
	// Chdir a tempdir random (no es el repo)
	empty := t.TempDir()
	originalCwd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalCwd) }()
	require.NoError(t, os.Chdir(empty))

	// Capturar stderr del runInstall
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() { os.Stderr = oldStderr }()

	// Correr runInstall con --non-interactive (evita prompt interactivo)
	exit := runInstall([]string{"--non-interactive"})

	// Cerrar write end y leer
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	stderr := buf.String()

	require.NotEqual(t, 0, exit, "runInstall debe abortar con exit != 0 fuera del repo")
	require.Contains(t, stderr, "no estás en el root del repo domain",
		"stderr debe contener mensaje claro del guard")
	require.True(t,
		strings.Contains(stderr, ".env.example") || strings.Contains(stderr, "docker-compose.yml"),
		"stderr debe mencionar los archivos faltantes")
}
