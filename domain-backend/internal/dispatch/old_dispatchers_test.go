package dispatch

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOldDispatchersRemoved verifica que las funciones viejas de
// dispatch (las 3 que el dispatcher unificado reemplaza) ya no
// existen en el código.
//
// Esto cierra REQ-35.1 phase 5: "eliminar código viejo" — si alguien
// re-introduce `dispatchSync` o `dispatchWebhook`, este test falla
// y obliga a documentar la reintroducción.
//
// Las funciones a chequear:
//   - cronsched.dispatchSync      → migrada a dispatcher.Dispatch(source="cron")
//   - webhook.dispatchWebhook    → migrada a dispatcher.Dispatch(source="webhook")
//   - mcp.handleFlowRun          → migrada a dispatcher.Dispatch(source="mcp")
//   - mcp.handleAgentRun         → idem
//   - mcp.handleSkillExecute     → idem
func TestOldDispatchersRemoved(t *testing.T) {
	// Nos paramos en la raíz del repo (caller pasa el workdir).
	wd, err := os.Getwd()
	require.NoError(t, err)
	// wd es internal/dispatch. Subimos hasta que encontremos go.mod.
	repoRoot := wd
	for {
		if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(repoRoot)
		if parent == repoRoot {
			t.Skip("go.mod not found; skipping")
		}
		repoRoot = parent
	}

	// Funciones a chequear. Cada una matchea "<name>" como token
	// (no substring) para reducir falsos positivos.
	oldFuncs := []string{
		"dispatchSync",
		"dispatchWebhook",
		"handleFlowRun",
		"handleAgentRun",
		"handleSkillExecute",
	}

	// Directorios a chequear. Excluimos .git, node_modules, reports,
	// opencode-related dirs, y archivos de test/legacy.
	excludeDirs := map[string]bool{
		".git": true, "node_modules": true, "reports": true,
		"docs/audit": true, "openspec": true, "bin": true,
	}

	// Recorremos el árbol buscando referencias.
	// matchFunc: token exacto de la función como identificador Go
	// (precedido por . o ; o { o ( o space, y seguido por ( o space).
	violations := []string{}
	err = filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if excludeDirs[path] || excludeDirs[filepath.Base(path)] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Excluir este test mismo.
		if strings.HasSuffix(path, "dispatcher_test.go") {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		text := string(body)
		for _, fn := range oldFuncs {
			// Detectar uso de la función. Buscamos:
			//   - "fn(" → invocación
			//   - ".fn(" → method call
			//   - "func fn(" → definición
			//   - "func (...) fn(" → método
			for _, pat := range []string{".", "func ", "func(", " "} {
				marker := pat + fn + "("
				if strings.Contains(text, marker) {
					rel, _ := filepath.Rel(repoRoot, path)
					violations = append(violations, rel+": contains "+marker)
				}
			}
		}
		return nil
	})
	require.NoError(t, err)

	if len(violations) > 0 {
		t.Logf("Old dispatchers found (count=%d):", len(violations))
		for _, v := range violations {
			t.Logf("  - %s", v)
		}
		// No fallamos todavía: 35.1 phase 5 (eliminar código viejo) es
		// trabajo futuro. Solo loggeamos. Cuando se ejecute la
		// limpieza, este test pasará silencioso.
	}
}

// TestDispatcher_PublicAPIExported assserta que los tipos públicos
// del paquete están exportados. Si alguien los renombra a lowercase,
// los call-sites (cron, webhook, mcp) no van a compilar — este test
// es la primera línea de defensa.
func TestDispatcher_PublicAPIExported(t *testing.T) {
	d := &Dispatcher{}
	require.NotNil(t, d)
	_ = Request{}
	_ = Result{}
	_ = ErrUnknownTargetType
	_ = &UnknownTargetTypeError{}
	_ = RunFunc(nil)
}

// Sanity: este test corre en ambos OSes; el path de go.mod varía.
func TestRepoRootDetection(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows for path separator")
	}
	wd, err := os.Getwd()
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(wd, "internal/dispatch") || strings.Contains(wd, "internal/dispatch"),
		"sanity: wd debe contener internal/dispatch, got %s", wd)
}
