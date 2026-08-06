package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DOMAINSERV-245, corrección de su cierre: el criterio "el marker registra el origen" se había
// declarado INALCANZABLE con el argumento de que ningún campo del payload identifica a un
// subagente. Es falso, y la documentación oficial de Claude Code lo dice textualmente:
//
//	agent_id — "Unique identifier for the subagent. Present only when the hook fires inside a
//	subagent call. Use this to distinguish subagent hook calls from main-thread calls."
//
// El error fue un salto: se midió que el session_id SE HEREDA (un subagente acuña el marker del
// padre, verificado con un experimento real) y de ahí se concluyó que nada discrimina — sin
// mirar el resto del payload. agent_id venía en cada invocación y ningún hook lo leía.
//
// Lo que estos tests fijan es el campo 6 del marker. NO cambia quién puede commitear: eso lo
// decide el alcance (campo 4, DOMAINSERV-237), y un subagente que corre la suite que cubre lo
// staged sigue habilitando el commit. Lo que cambia es que ahora queda auditable QUIÉN produjo
// la evidencia.

func campoDelMarker(t *testing.T, home string, campo int) string {
	t.Helper()
	ms, err := filepath.Glob(filepath.Join(home, ".local", "state", "domain", "tests-ok-*"))
	if err != nil || len(ms) == 0 {
		t.Fatalf("no hay marker para leer: %v", err)
	}
	b, err := os.ReadFile(ms[0])
	if err != nil {
		t.Fatalf("leer marker: %v", err)
	}
	linea := strings.SplitN(strings.TrimRight(string(b), "\n"), "\n", 2)[0]
	campos := strings.Split(linea, "\t")
	if len(campos) < campo {
		t.Fatalf("el marker tiene %d campos y se pidió el %d: %q", len(campos), campo, linea)
	}
	return campos[campo-1]
}

func TestMarker_CorridaDelHiloPrincipal_RegistraOrigenMain(t *testing.T) {
	dir := repoGitDePrueba(t)
	escribirArchivo(t, dir, "main.go", "package main\n\nfunc main() { println(1) }\n")
	gitFixture(t, dir, "add", "-A")

	home := t.TempDir()
	// payloadCorrida NO incluye agent_id, que es exactamente el caso del hilo principal
	correrHookEnRepo(t, "domain-post-test.sh", home, dir,
		payloadCorrida(t, "go test -count=1 ./...", "ok  domain  0.2s"))

	if got := campoDelMarker(t, home, 6); got != "main" {
		t.Errorf("el origen del hilo principal quedó como %q y se esperaba \"main\": sin este campo "+
			"un marker acuñado por un subagente es indistinguible de uno propio", got)
	}
}

func TestMarker_CorridaDeUnSubagente_RegistraSuAgentIdYTipo(t *testing.T) {
	dir := repoGitDePrueba(t)
	escribirArchivo(t, dir, "main.go", "package main\n\nfunc main() { println(2) }\n")
	gitFixture(t, dir, "add", "-A")

	home := t.TempDir()
	payload := payloadJSON(t, map[string]any{
		"session_id":      sesionGate, // el MISMO del padre: el subagente lo hereda
		"hook_event_name": "PostToolUse",
		"tool_name":       "Bash",
		"agent_id":        "a95730c314368c489",
		"agent_type":      "general-purpose",
		"tool_input":      map[string]any{"command": "go test -count=1 ./..."},
		"tool_response":   map[string]any{"stdout": "ok  domain  0.2s", "stderr": "", "interrupted": false},
	})
	correrHookEnRepo(t, "domain-post-test.sh", home, dir, payload)

	got := campoDelMarker(t, home, 6)
	if !strings.Contains(got, "a95730c314368c489") {
		t.Errorf("el origen no incluye el agent_id: %q. Es el único dato que distingue una corrida "+
			"de subagente, porque el session_id es el del padre", got)
	}
	if !strings.Contains(got, "general-purpose") {
		t.Errorf("el origen no incluye el agent_type, que es lo legible para quien lee el deny: %q", got)
	}
	if got == "main" {
		t.Error("una corrida CON agent_id quedó registrada como \"main\": el campo no se está leyendo")
	}
}

// El campo 6 es NUEVO y va al final a propósito: los lectores existentes usan cut -f1..f5. Si
// alguien lo insertara en el medio, el gate leería el alcance de la columna equivocada y
// denegaría commits legítimos — el modo de falla de DOMAINSERV-111/175/195.
func TestMarker_ElOrigenNoDesplazaLosCamposQueElGateYaLeia(t *testing.T) {
	dir := repoGitDePrueba(t)
	escribirArchivo(t, dir, "main.go", "package main\n\nfunc main() { println(3) }\n")
	gitFixture(t, dir, "add", "-A")

	home := t.TempDir()
	correrHookEnRepo(t, "domain-post-test.sh", home, dir,
		payloadCorrida(t, "go test -count=1 ./...", "ok  domain  0.3s"))

	// campo 5 = runner, el último que existía antes de este cambio
	if got := campoDelMarker(t, home, 5); got != "go" {
		t.Errorf("el campo 5 (runner) dejó de ser \"go\" y quedó %q: el origen se insertó en el "+
			"medio y desplazó lo que el gate ya leía", got)
	}
	// y el alcance sigue en el 4
	if got := campoDelMarker(t, home, 4); got == "" {
		t.Error("el campo 4 (alcance) quedó vacío: el orden de los campos se rompió")
	}

	// y el commit sigue habilitado, que es la prueba de que nada de esto cambió la decisión
	out := correrHookEnRepo(t, "domain-pre-edit.sh", home, dir, payloadIntentoDeCommit(t))
	if strings.Contains(out, `"deny"`) {
		t.Errorf("agregar el campo de origen rompió un commit que antes pasaba: %s", out)
	}
}
