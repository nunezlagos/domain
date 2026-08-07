package mcpserver

import (
	"os"
	"strings"
	"testing"
)

// REQ-2 de issue-57.1: el hook ya hace un round-trip al VPS en cada sesión, así que la
// versión del server viaja gratis en ese response. Sin esto el cliente no tiene contra qué
// compararse y no hay forma de que nadie se entere de que quedó atrás.

func TestInfoDeVersionDelServer_ConVersionInyectada_ExponeVersionYMinimoDeCliente(t *testing.T) {
	h := &sessionBootstrapHandlers{serverVersion: "1.4.2"}

	info := h.infoDeVersionDelServer()

	if got := info["version"]; got != "1.4.2" {
		t.Errorf("version = %v, esperaba la versión con la que se compiló el server", got)
	}
	if _, hay := info["min_client_version"]; !hay {
		t.Error("falta min_client_version: el cliente no puede distinguir 'viejo' de 'ya no soportado' (REQ-6)")
	}
}

// Un server sin versión inyectada no puede fingir un número: el cliente lo compararía y le
// avisaría al usuario que actualice contra una versión que no existe.
func TestInfoDeVersionDelServer_SinVersionInyectada_NoFingeUnNumero(t *testing.T) {
	h := &sessionBootstrapHandlers{serverVersion: ""}

	if got := h.infoDeVersionDelServer()["version"]; got != "" {
		t.Errorf("version = %v; sin inyección el campo debe quedar vacío, no con un número inventado", got)
	}
}

// El piso de compatibilidad se declara vacío hasta que la fase 4 lo defina. Vacío significa
// "sin piso": si algún día se llena con algo que no sea una versión ordenable, el cliente lo
// descartaría en silencio y REQ-6 quedaría roto sin que nada lo note.
func TestVersionMinimaDeCliente_SiSeDeclara_TieneFormatoOrdenable(t *testing.T) {
	if versionMinimaDeCliente == "" {
		return
	}
	for _, parte := range strings.Split(strings.TrimPrefix(versionMinimaDeCliente, "v"), ".") {
		if parte == "" || strings.ContainsFunc(parte, func(r rune) bool { return r < '0' || r > '9' }) {
			t.Fatalf("versionMinimaDeCliente = %q: el cliente solo sabe ordenar números separados por puntos, así que este piso no se aplicaría nunca", versionMinimaDeCliente)
		}
	}
}

// Guard sobre el guard, mismo patrón que TestValidarParticionDisjunta_TieneCallerEnProduccion:
// una función que arma el bloque de versión pero que el handler no llama deja el REQ-2
// incumplido con su test unitario en verde.
func TestInfoDeVersionDelServer_TieneCallerEnElHandlerDelBootstrap(t *testing.T) {
	raw, err := os.ReadFile("session_bootstrap_tools.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "infoDeVersionDelServer()") {
		t.Fatal("el handler del bootstrap no llama a infoDeVersionDelServer: la versión no llegaría al cliente (policy guards-deben-ejecutarse)")
	}
}
