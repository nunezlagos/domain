package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// DOMAINSERV-219: el commit-gate invalidaba el marker de tests con el hash de
// `git diff HEAD`, que cambia por dos motivos ajenos al código: tocar un archivo
// inerte (CHANGELOG.md) y commitear (HEAD se mueve). Estos tests miran la
// decisión REAL del hook corriendo sobre un repo git de verdad.
//
// Las contra-pruebas son la mitad del test: los .md de install-user/templates
// están en go:embed (embed.go:16) y son ASSERTS de agent_voseo_test.go, así que
// un "*.md es inerte" a secas bendice un falso verde.

const sesionGate = "sesion-commit-gate"

// gitFixture corre git SIEMPRE con -C sobre el repo de prueba, y con user.name/
// user.email inline porque el runner de CI no tiene identidad global configurada.
func gitFixture(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir,
		"-c", "user.email=test@domain.local", "-c", "user.name=test"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func repoGitDePrueba(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	archivos := map[string]string{
		// DOMAINSERV-237: el go.mod NO es decorativo. El alcance del marker se ancla al
		// módulo que gobierna el path, porque un path sin go.mod no pudo ser recorrido
		// por ninguna corrida de Go y registrarlo afirmaría una cobertura inexistente.
		// Sin este archivo el fixture no representa un módulo y los casos de alcance
		// medirían otra cosa.
		"go.mod":                         "module fixture\n\ngo 1.22\n",
		"main.go":                        "package main\n\nfunc main() {}\n",
		"CHANGELOG.md":                   "# Changelog\n",
		"services/install.sh":            "#!/usr/bin/env bash\necho hola\n",
		"templates/agents/repo-scout.md": "---\nname: repo-scout\n---\ncuerpo\n",
	}
	for rel, contenido := range archivos {
		abs := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(contenido), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitFixture(t, dir, "init", "-q")
	gitFixture(t, dir, "add", "-A")
	gitFixture(t, dir, "commit", "-q", "-m", "inicial")
	return dir
}

func hookAbsoluto(t *testing.T, script string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("hooks", script))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// correrHookEnRepo corre el hook con cwd en el repo de prueba: el gate consulta
// git sobre el cwd, así que sin cmd.Dir el hook miraría este repo y no el fixture.
func correrHookEnRepo(t *testing.T, script, home, dir, payload string) string {
	t.Helper()
	cmd := exec.Command("bash", hookAbsoluto(t, script))
	cmd.Stdin = strings.NewReader(payload)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s falló: %v", script, err)
	}
	return string(out)
}

func payloadCorridaVerde(t *testing.T) string {
	t.Helper()
	return payloadJSON(t, map[string]any{
		"session_id":      sesionGate,
		"hook_event_name": "PostToolUse",
		"tool_name":       "Bash",
		// DOMAINSERV-237: el -count=1 pasó a ser parte del contrato de qué corrida
		// vale como prueba — sin él la suite puede venir entera del cache. Antes
		// esta fixture decía `go test ./...` y con el contrato nuevo deja de
		// escribir marker, así que los 3 tests de DOMAINSERV-219 fallaban por el
		// motivo equivocado: no porque su invariante se rompiera, sino porque su
		// corrida verde ya no califica.
		"tool_input":    map[string]any{"command": "go test -count=1 ./..."},
		"tool_response": map[string]any{"stdout": "ok  domain 0.4s", "stderr": "", "interrupted": false},
	})
}

func payloadIntentoDeCommit(t *testing.T) string {
	t.Helper()
	return payloadJSON(t, map[string]any{
		"session_id":      sesionGate,
		"hook_event_name": "PreToolUse",
		"tool_name":       "Bash",
		"permission_mode": "bypassPermissions",
		"tool_input":      map[string]any{"command": "git commit -m mensaje"},
	})
}

func payloadJSON(t *testing.T, m map[string]any) string {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// corridaVerdeYMarker deja el marker tests-ok escrito por el hook real (no
// forjado a mano: el formato del marker es justo lo que está en discusión).
func corridaVerdeYMarker(t *testing.T, dir string) string {
	t.Helper()
	home := t.TempDir()
	correrHookEnRepo(t, "domain-post-test.sh", home, dir, payloadCorridaVerde(t))
	marker := filepath.Join(home, ".local", "state", "domain", "tests-ok-"+sesionGate)
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("el post-test no dejó el marker: %v", err)
	}
	return home
}

func escribirArchivo(t *testing.T, dir, rel, contenido string) {
	t.Helper()
	abs := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(contenido), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCommitGate_MarkerFresco_SoloCambiaMarkdownInerte_DejaCommitear(t *testing.T) {
	dir := repoGitDePrueba(t)
	home := corridaVerdeYMarker(t, dir)

	escribirArchivo(t, dir, "CHANGELOG.md", "# Changelog\n\n- registro del fix\n")

	out := correrHookEnRepo(t, "domain-pre-edit.sh", home, dir, payloadIntentoDeCommit(t))
	if strings.Contains(out, "permissionDecision") {
		t.Errorf("el gate exigió re-correr la suite por editar CHANGELOG.md: %s", out)
	}
}

// Secuencia real: se implementa el fix, se corre la suite en verde, se commitea
// (eso el gate lo permite) y el segundo commit del mismo lote vuelve a pasar por
// el gate con el MISMO código. `git diff HEAD` cambió —de tener el fix a estar
// vacío— aunque el árbol es idéntico al que se testeó.
func TestCommitGate_MarkerFresco_SegundoCommitSinTocarCodigo_DejaCommitear(t *testing.T) {
	dir := repoGitDePrueba(t)
	escribirArchivo(t, dir, "main.go", "package main\n\nfunc main() { println(2) }\n")
	home := corridaVerdeYMarker(t, dir)

	gitFixture(t, dir, "add", "-A")
	gitFixture(t, dir, "commit", "-q", "-m", "el fix ya testeado")

	out := correrHookEnRepo(t, "domain-pre-edit.sh", home, dir, payloadIntentoDeCommit(t))
	if strings.Contains(out, "permissionDecision") {
		t.Errorf("mover HEAD invalidó una corrida que sigue cubriendo el código: %s", out)
	}
}

// El corazón del fix: la lista de inertes NO puede tragarse nada que un test
// pueda romper. Cada caso de acá tiene que seguir exigiendo la suite.
func TestCommitGate_MarkerFresco_CambioEnCodigo_SigueBloqueando(t *testing.T) {
	casos := []struct {
		nombre    string
		archivo   string
		contenido string
	}{
		{"go trackeado", "main.go", "package main\n\nfunc main() { println(1) }\n"},
		{"shell", "services/install.sh", "#!/usr/bin/env bash\necho chau\n"},
		{"md embebido en go:embed", "templates/agents/repo-scout.md", "---\nname: repo-scout\n---\notro cuerpo\n"},
		{"go nuevo sin git add", "nuevo.go", "package main\n\nfunc nuevo() {}\n"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			dir := repoGitDePrueba(t)
			home := corridaVerdeYMarker(t, dir)

			escribirArchivo(t, dir, c.archivo, c.contenido)

			out := correrHookEnRepo(t, "domain-pre-edit.sh", home, dir, payloadIntentoDeCommit(t))
			if !strings.Contains(out, `"deny"`) || !strings.Contains(out, "commit-gate") {
				t.Errorf("%s cambió después de la corrida y el gate dejó commitear (falso verde): %s", c.archivo, out)
			}
		})
	}
}
