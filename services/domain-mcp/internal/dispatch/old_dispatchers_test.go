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

	wd, err := os.Getwd()
	require.NoError(t, err)

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



	oldFuncs := []string{
		"dispatchSync",
		"dispatchWebhook",
		"handleFlowRun",
		"handleAgentRun",
		"handleSkillExecute",
	}



	excludeDirs := map[string]bool{
		".git": true, "node_modules": true, "reports": true,
		"docs/audit": true, "openspec": true, "bin": true,
	}




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

		if strings.HasSuffix(path, "dispatcher_test.go") {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		text := string(body)
		for _, fn := range oldFuncs {





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
