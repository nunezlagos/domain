package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Hook Stop, portado de hooks/domain-stop.sh (DOMAINSERV-273, piloto del epic).
//
// Cierra el turno en domain con el prompt_id que dejó el hook UserPromptSubmit, y de paso
// hace la higiene de markers del turno.
//
// TODO el hook es best-effort: SIEMPRE sale 0. Un lifecycle hook que falla bloquea la sesión
// del usuario, y ningún dato de métricas justifica eso.

// ejecutarHook despacha los lifecycle hooks que ya están portados a Go. Un nombre que no
// conoce devuelve 0 en vez de error: durante el port conviven hooks en .sh y en Go, y un
// binario viejo que reciba un subcomando nuevo no puede romper la sesión por eso.
func ejecutarHook(nombre string, entrada io.Reader, home string) int {
	switch nombre {
	case "stop":
		return ejecutarHookStop(entrada, home)
	default:
		return 0
	}
}

type entradaHookStop struct {
	SessionID            string `json:"session_id"`
	StopHookActive       bool   `json:"stop_hook_active"`
	LastAssistantMessage string `json:"last_assistant_message"`
}

func ejecutarHookStop(entrada io.Reader, home string) int {
	crudo, err := io.ReadAll(entrada)
	if err != nil {
		return 0
	}
	var p entradaHookStop
	if err := json.Unmarshal(crudo, &p); err != nil {
		return 0
	}

	// stop_hook_active: el propio Stop puede disparar otro Stop. Salir acá es lo único que
	// evita el loop, y va antes que cualquier otra cosa.
	if p.StopHookActive || p.SessionID == "" {
		return 0
	}

	estado := filepath.Join(home, ".local", "state", "domain")

	// el marker de tests muere con el turno: el commit-gate exige una corrida que cubra el
	// estado ACTUAL del código, y el turno siguiente ya no lo garantiza (DOMAINSERV-74)
	os.Remove(filepath.Join(estado, "tests-ok-"+p.SessionID))

	podarMarkersDeFlow(estado)
	cerrarTurno(estado, home, p)
	return 0
}

// podarMarkersDeFlow borra los markers flow-* de más de 24h.
//
// NO se borran los del turno, y es deliberado (DOMAINSERV-181): un flow SDD dura varios turnos
// por diseño —el modo hybrid pausa esperando al humano, o sea que GARANTIZA cruzar de turno— y
// borrarlos dejaba al agente sin poder editar con el flow todavía corriendo. No es un agujero:
// el marker local no es la autoridad, el gate revalida el token contra el server en cada
// edición, y el HMAC ya venció mucho antes de las 24h.
func podarMarkersDeFlow(estado string) {
	entradas, err := os.ReadDir(estado)
	if err != nil {
		return
	}
	limite := time.Now().Add(-24 * time.Hour)
	for _, e := range entradas {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "flow-") {
			continue
		}
		info, err := e.Info()
		if err != nil || info.ModTime().After(limite) {
			continue
		}
		os.Remove(filepath.Join(estado, e.Name()))
	}
}

// cerrarTurno reporta el turno a domain. Solo procede si el hook UserPromptSubmit dejó un
// turn-id: sin prompt capturado no hay turno que cerrar.
func cerrarTurno(estado, home string, p entradaHookStop) {
	idFile := filepath.Join(estado, "turn-"+p.SessionID+".id")
	crudo, err := os.ReadFile(idFile)
	if err != nil {
		return
	}
	// se borra apenas se lee: si el cierre falla, el turno no queda reintentándose para
	// siempre en cada Stop de la sesión
	os.Remove(idFile)

	promptID := strings.TrimSpace(string(crudo))
	if promptID == "" {
		return
	}

	// las credenciales recién hacen falta acá: es lo único que sale a la red
	cred, ok := resolverCredenciales(home)
	if !ok {
		return
	}

	args := map[string]any{
		"prompt_id": promptID,
		// aproximación del largo de la respuesta: no incluye tool calls, y alcanza para
		// métricas de uso
		"response_chars": len(p.LastAssistantMessage),
	}
	resp, err := llamarTool(cred, "domain_turn_complete", args, 6*time.Second)
	if err != nil || strings.Contains(resp, `"error"`) {
		detalle := resp
		if err != nil {
			detalle = err.Error() + " " + resp
		}
		registrarErrorDeHook(home, "Stop", p.SessionID, "domain_turn_complete", detalle)
	}
}
