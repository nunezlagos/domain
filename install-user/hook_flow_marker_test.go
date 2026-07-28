package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// DOMAINSERV-181: claude_hook_matcher_test.go verifica que el matcher NOMBRE las
// tools, pero no que el hook haga algo con ellas. El matcher estaba bien y el
// gate igual se caía: el hook corría y salía sin escribir el marker porque el
// response de confirm/phase_result no trae flow_run_id. Estos tests miran el
// efecto en disco, no la declaración.

const flowRunDePrueba = "a99e5b84-7864-43e7-bae4-073cbb564df9"

// correrHook ejecuta un hook con HOME apuntando a un tmpdir. Sin credenciales
// resueltas el post-orchestrate cae a su formato legacy, que escribe el marker
// igual: alcanza para verificar que NO salió temprano.
func correrHook(t *testing.T, script, home, payload string) {
	t.Helper()
	cmd := exec.Command("bash", filepath.Join("hooks", script))
	cmd.Stdin = stringReader(payload)
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s falló: %v\n%s", script, err, out)
	}
}

func stringReader(s string) *os.File {
	r, w, _ := os.Pipe()
	go func() {
		_, _ = w.WriteString(s)
		_ = w.Close()
	}()
	return r
}

func markerDeFlow(home, session string) string {
	return filepath.Join(home, ".local", "state", "domain", "flow-"+session)
}

func payloadPostOrchestrate(t *testing.T, session, tool string, respuesta map[string]any) string {
	t.Helper()
	texto, err := json.Marshal(respuesta)
	if err != nil {
		t.Fatal(err)
	}
	p, err := json.Marshal(map[string]any{
		"session_id":      session,
		"hook_event_name": "PostToolUse",
		"tool_name":       tool,
		"tool_input":      map[string]any{},
		"tool_response":   []any{map[string]any{"type": "text", "text": string(texto)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(p)
}

// Las 4 tools que abren o avanzan un flow tienen que dejar el marker escrito.
// confirm y phase_result devuelven PascalCase: son las que se usan al RETOMAR,
// justo donde el gate se caía.
func TestPostOrchestrate_ToolQueAvanzaElFlow_EscribeElMarker(t *testing.T) {
	casos := []struct {
		tool      string
		respuesta map[string]any
	}{
		{"mcp__domain-mcp__domain_orchestrate", map[string]any{"flow_run_id": flowRunDePrueba, "mode": "lite"}},
		{"mcp__domain-mcp__domain_flow_status", map[string]any{"flow_run_id": flowRunDePrueba, "mode": "lite"}},
		{"mcp__domain-mcp__domain_orchestrate_phase_result", map[string]any{
			"StepID": "47506461-9418-43d1-90ea-9d789d457086", "StepStatus": "completed",
			"FlowRunStatus": "running", "FlowRunID": flowRunDePrueba,
		}},
		{"mcp__domain-mcp__domain_orchestrate_confirm", map[string]any{
			"StepID": "47506461-9418-43d1-90ea-9d789d457086", "StepStatus": "pending",
			"FlowRunStatus": "running", "FlowRunID": flowRunDePrueba,
		}},
	}
	for _, c := range casos {
		t.Run(c.tool, func(t *testing.T) {
			home := t.TempDir()
			session := "sesion-de-prueba"
			correrHook(t, "domain-post-orchestrate.sh", home, payloadPostOrchestrate(t, session, c.tool, c.respuesta))
			if _, err := os.Stat(markerDeFlow(home, session)); err != nil {
				t.Errorf("%s no dejó el marker del gate: %v — el agente no puede editar aunque el flow esté activo", c.tool, err)
			}
		})
	}
}

func TestPostOrchestrate_FlowCancel_BorraElMarker(t *testing.T) {
	home := t.TempDir()
	session := "sesion-de-prueba"
	escribirMarker(t, markerDeFlow(home, session), time.Now())

	correrHook(t, "domain-post-orchestrate.sh", home,
		payloadPostOrchestrate(t, session, "mcp__domain-mcp__domain_flow_cancel", map[string]any{"flow_run_id": flowRunDePrueba}))

	if _, err := os.Stat(markerDeFlow(home, session)); !os.IsNotExist(err) {
		t.Error("cancelar el flow tiene que retirar la autorización de edición")
	}
}

func escribirMarker(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("token-de-prueba\t2099-01-01T00:00:00Z\tlite\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func payloadStop(t *testing.T, session string) string {
	t.Helper()
	p, err := json.Marshal(map[string]any{
		"session_id":       session,
		"hook_event_name":  "Stop",
		"stop_hook_active": false,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(p)
}

// El corazón de DOMAINSERV-181: un flow SDD dura varios turnos por diseño, y el
// modo hybrid GARANTIZA cruzar de turno porque pausa para el humano.
func TestStop_MarkerDeFlowVigente_SobreviveElCierreDeTurno(t *testing.T) {
	home := t.TempDir()
	session := "sesion-de-prueba"
	marker := markerDeFlow(home, session)
	escribirMarker(t, marker, time.Now())

	correrHook(t, "domain-stop.sh", home, payloadStop(t, session))

	if _, err := os.Stat(marker); err != nil {
		t.Errorf("el Stop borró la autorización de un flow vigente: %v — al retomar, el gate deniega toda edición", err)
	}
}

// tests-ok sí es por turno: el commit-gate exige una corrida que cubra el estado
// actual del código, no una de hace tres turnos.
func TestStop_MarkerDeTests_SeLimpiaPorTurno(t *testing.T) {
	home := t.TempDir()
	session := "sesion-de-prueba"
	testsOK := filepath.Join(home, ".local", "state", "domain", "tests-ok-"+session)
	escribirMarker(t, testsOK, time.Now())

	correrHook(t, "domain-stop.sh", home, payloadStop(t, session))

	if _, err := os.Stat(testsOK); !os.IsNotExist(err) {
		t.Error("el marker de tests tiene que morir con el turno")
	}
}

// La higiene que DOMAINSERV-78 buscaba: los huérfanos de sesiones muertas.
func TestStop_MarkersHuerfanosViejos_SePodan(t *testing.T) {
	home := t.TempDir()
	session := "sesion-de-prueba"
	viejo := markerDeFlow(home, "sesion-muerta-hace-dias")
	escribirMarker(t, viejo, time.Now().Add(-48*time.Hour))
	actual := markerDeFlow(home, session)
	escribirMarker(t, actual, time.Now())

	correrHook(t, "domain-stop.sh", home, payloadStop(t, session))

	if _, err := os.Stat(viejo); !os.IsNotExist(err) {
		t.Error("un marker de hace 48h es de una sesión que ya no existe: hay que podarlo")
	}
	if _, err := os.Stat(actual); err != nil {
		t.Errorf("la poda se llevó puesto el marker vigente: %v", err)
	}
}

func TestStop_SinMarkers_NoFalla(t *testing.T) {
	home := t.TempDir()
	correrHook(t, "domain-stop.sh", home, payloadStop(t, "sesion-sin-estado"))
	fmt.Fprintln(os.Stderr, "")
}
