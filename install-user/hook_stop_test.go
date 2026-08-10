package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// DOMAINSERV-273, piloto del port de hooks a Go. Estos tests fijan los invariantes que el
// domain-stop.sh documentaba en comentarios y que nada verificaba: la suite de shell del repo
// no cubría este hook.
//
// Lo que hace valioso al port no es el lenguaje, es esto: en .sh estos invariantes eran
// afirmaciones en un comentario; acá fallan si alguien los rompe.

func estadoDomain(t *testing.T, home string) string {
	t.Helper()
	dir := filepath.Join(home, ".local", "state", "domain")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func payloadStopJSON(t *testing.T, sesion string, stopActivo bool, respuesta string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"session_id":             sesion,
		"stop_hook_active":       stopActivo,
		"last_assistant_message": respuesta,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// El invariante más importante del hook: un Stop puede disparar otro Stop, y sin este corte
// el hook se llama a sí mismo. Va antes que cualquier otra cosa, incluida la higiene.
func TestHookStop_StopHookActive_SaleSinTocarNada(t *testing.T) {
	home := t.TempDir()
	estado := estadoDomain(t, home)
	marker := filepath.Join(estado, "tests-ok-sesion-1")
	if err := os.WriteFile(marker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	rc := ejecutarHookStop(strings.NewReader(payloadStopJSON(t, "sesion-1", true, "hola")), home)

	if rc != 0 {
		t.Fatalf("un hook nunca puede salir != 0: bloquearía la sesión (rc=%d)", rc)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Error("con stop_hook_active el hook hizo trabajo igual: el corte tiene que ser lo primero")
	}
}

// El marker de tests muere con el turno: el commit-gate exige una corrida que cubra el estado
// ACTUAL del código, y el turno siguiente ya no lo garantiza (DOMAINSERV-74).
func TestHookStop_BorraElMarkerDeTestsDelTurno(t *testing.T) {
	home := t.TempDir()
	estado := estadoDomain(t, home)
	mio := filepath.Join(estado, "tests-ok-sesion-1")
	ajeno := filepath.Join(estado, "tests-ok-otra-sesion")
	for _, f := range []string{mio, ajeno} {
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	ejecutarHookStop(strings.NewReader(payloadStopJSON(t, "sesion-1", false, "")), home)

	if _, err := os.Stat(mio); err == nil {
		t.Error("no borró el marker de tests del turno: el commit-gate lo aceptaría en el turno " +
			"siguiente, sobre código que esa corrida ya no cubre")
	}
	if _, err := os.Stat(ajeno); err != nil {
		t.Error("borró el marker de OTRA sesión: dos sesiones en paralelo se pisarían entre sí")
	}
}

// DOMAINSERV-181, y es el invariante que más caro salió aprender: un flow SDD dura VARIOS
// turnos por diseño —el modo hybrid pausa esperando al humano— así que borrar sus markers por
// turno dejaba al agente sin poder editar con el flow todavía corriendo. Se podan por
// antigüedad, no por turno.
func TestHookStop_NoBorraElMarkerDeFlowDelTurno(t *testing.T) {
	home := t.TempDir()
	estado := estadoDomain(t, home)
	reciente := filepath.Join(estado, "flow-sesion-1")
	viejo := filepath.Join(estado, "flow-sesion-vieja")
	for _, f := range []string{reciente, viejo} {
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	hace48h := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(viejo, hace48h, hace48h); err != nil {
		t.Fatal(err)
	}

	ejecutarHookStop(strings.NewReader(payloadStopJSON(t, "sesion-1", false, "")), home)

	if _, err := os.Stat(reciente); err != nil {
		t.Error("borró el marker de flow del turno: un flow SDD cruza turnos por diseño, y sin " +
			"marker el agente no puede editar con el flow todavía running (DOMAINSERV-181)")
	}
	if _, err := os.Stat(viejo); err == nil {
		t.Error("no podó el marker de flow de hace 48h: los huérfanos se acumulan para siempre")
	}
}

// Sin turn-id no hay turno que cerrar, y el hook no puede inventarlo ni salir a la red.
func TestHookStop_SinTurnID_NoSaleALaRed(t *testing.T) {
	home := t.TempDir()
	estadoDomain(t, home)
	llamado := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		llamado = true
	}))
	defer srv.Close()
	t.Setenv("DOMAIN_VPS_URL", srv.URL)
	t.Setenv("DOMAIN_API_KEY", "k")

	ejecutarHookStop(strings.NewReader(payloadStopJSON(t, "sesion-1", false, "hola")), home)

	if llamado {
		t.Error("salió a la red sin prompt capturado: no hay turno que cerrar")
	}
}

func TestHookStop_ConTurnID_CierraElTurnoYConsumeElID(t *testing.T) {
	home := t.TempDir()
	estado := estadoDomain(t, home)
	idFile := filepath.Join(estado, "turn-sesion-1.id")
	if err := os.WriteFile(idFile, []byte("prompt-abc\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var recibido map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer la-key" {
			t.Errorf("Authorization mal armado: %q", got)
		}
		_ = json.NewDecoder(r.Body).Decode(&recibido)
		w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{}}`))
	}))
	defer srv.Close()
	t.Setenv("DOMAIN_VPS_URL", srv.URL)
	t.Setenv("DOMAIN_API_KEY", "la-key")

	ejecutarHookStop(strings.NewReader(payloadStopJSON(t, "sesion-1", false, "12345")), home)

	if recibido == nil {
		t.Fatal("no se llamó a domain_turn_complete con un turn-id presente")
	}
	params, _ := recibido["params"].(map[string]any)
	if params == nil || params["name"] != "domain_turn_complete" {
		t.Fatalf("llamó al tool equivocado: %v", recibido)
	}
	args, _ := params["arguments"].(map[string]any)
	if args["prompt_id"] != "prompt-abc" {
		t.Errorf("prompt_id incorrecto: %v (¿se limpió el salto de línea del archivo?)", args["prompt_id"])
	}
	if args["response_chars"] != float64(5) {
		t.Errorf("response_chars incorrecto: %v", args["response_chars"])
	}

	// el id se consume: si el cierre falla, el turno no puede quedar reintentándose en cada
	// Stop de la sesión
	if _, err := os.Stat(idFile); err == nil {
		t.Error("el turn-id sobrevivió: se reintentaría el mismo turno para siempre")
	}
}

// Un server caído no puede tumbar la sesión, pero tampoco puede ser invisible: sin rastro en
// hook-errors.log, un fallo del turn_complete no lo nota nadie (REQ-56 issue-56.2).
func TestHookStop_ServerQueFalla_NoBloqueaPeroDejaRastro(t *testing.T) {
	home := t.TempDir()
	estado := estadoDomain(t, home)
	if err := os.WriteFile(filepath.Join(estado, "turn-sesion-1.id"), []byte("prompt-abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	t.Setenv("DOMAIN_VPS_URL", srv.URL)
	t.Setenv("DOMAIN_API_KEY", "k")

	rc := ejecutarHookStop(strings.NewReader(payloadStopJSON(t, "sesion-1", false, "x")), home)

	if rc != 0 {
		t.Fatalf("un server caído hizo fallar el hook (rc=%d): eso bloquea la sesión del usuario", rc)
	}
	log, err := os.ReadFile(filepath.Join(estado, "hook-errors.log"))
	if err != nil {
		t.Fatal("no se registró el fallo: un turn_complete que no llega es invisible")
	}
	if !strings.Contains(string(log), "domain_turn_complete") {
		t.Errorf("el log no nombra la operación que falló: %s", log)
	}
	if strings.Count(strings.TrimSpace(string(log)), "\n") != 0 {
		t.Error("el error ocupa más de una línea: el formato TSV del log deja de ser parseable")
	}
}

// Basura en la entrada no puede tumbar la sesión: el hook lee lo que le manda el cliente.
func TestHookStop_EntradaInvalida_SaleCero(t *testing.T) {
	home := t.TempDir()
	for _, entrada := range []string{"", "no soy json", "{", "null"} {
		if rc := ejecutarHookStop(strings.NewReader(entrada), home); rc != 0 {
			t.Errorf("entrada %q hizo salir != 0 (rc=%d)", entrada, rc)
		}
	}
}

// Un subcomando desconocido no puede romper la sesión: durante el port conviven hooks en .sh y
// en Go, y un binario viejo puede recibir un nombre que todavía no conoce.
func TestEjecutarHook_NombreDesconocido_SaleCero(t *testing.T) {
	if rc := ejecutarHook("todavia-no-existe", strings.NewReader("{}"), t.TempDir()); rc != 0 {
		t.Errorf("un hook desconocido salió != 0 (rc=%d)", rc)
	}
}
