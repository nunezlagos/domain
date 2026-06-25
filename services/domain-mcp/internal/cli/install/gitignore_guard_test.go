// Gitignore guard para opencode.json y .mcp.json (issue-29.4).
//
// El 2026-06-12 se commiteó accidentalmente opencode.json (con
// paths absolutos del home). El archivo se removió del index y
// se agregó al .gitignore. Este test blinda esa decisión: si
// alguien borra las entradas del .gitignore o fuerza `git add -f`,
// el test falla.
package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOpencodeJSONNotTracked verifica que opencode.json NO está
// tracked en git (debe estar en .gitignore).
//
// Si el archivo vuelve a ser tracked (e.g. `git add -f`), este
// test DEBE FALLAR.
func TestOpencodeJSONNotTracked(t *testing.T) {
	repoRoot := findRepoRootFromCwd(t)
	assertNotTracked(t, repoRoot, "opencode.json")
	assertNotTracked(t, repoRoot, ".mcp.json")
}

// TestGitignoreHasLocalConfigEntries verifica que las 4 entradas
// (opencode.json, opencode.json.backup-*, .mcp.json, .mcp.json.backup-*)
// siguen en .gitignore.
//
// Si alguien remueve las entradas (sabotaje), este test DEBE FALLAR.
func TestGitignoreHasLocalConfigEntries(t *testing.T) {
	repoRoot := findRepoRootFromCwd(t)
	gitignorePath := filepath.Join(repoRoot, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	require.NoError(t, err, "no se pudo leer .gitignore")
	content := string(data)

	required := []string{
		"opencode.json",
		"opencode.json.backup-*",
		".mcp.json",
		".mcp.json.backup-*",
	}
	for _, entry := range required {
		require.True(t, strings.Contains(content, entry),
			".gitignore debe contener %q (sabotaje: alguien la borró)", entry)
	}
}

// findRepoRootFromCwd walks up from cwd hasta encontrar un
// directorio con .git (es el root del repo). Falla el test si
// no lo encuentra.
func findRepoRootFromCwd(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (.git) walking up from cwd")
		}
		dir = parent
	}
	t.Fatal("walked up 10 levels without finding .git")
	return ""
}

// assertNotTracked corre `git ls-files --error-unmatch <path>` y
// falla si el comando retorna exit 0 (lo que indicaría que el
// archivo está tracked).
func assertNotTracked(t *testing.T, repoRoot, path string) {
	t.Helper()
	cmd := exec.Command("git", "ls-files", "--error-unmatch", path)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("%s está tracked en git (NO debería):\n%s", path, out)
	}


	_ = out
}
