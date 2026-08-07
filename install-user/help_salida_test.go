package main

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// DOMAINSERV-251. `flag.Usage = printHelp` y printHelp escribía con fmt.Println, o sea a
// STDOUT: ante un flag INVÁLIDO el paquete flag dispara el usage y las ~30 líneas de ayuda
// salen por la misma salida que parsea hooks/domain-session-start.sh.
//
// El matiz que separa los dos casos: un --help PEDIDO por el usuario es la salida legítima del
// programa y va a stdout con exit 0 (convención Unix); un usage disparado por un error de uso
// es diagnóstico y va a stderr. Mandar todo a stderr "arreglaría" el hook rompiendo el caso
// legítimo, así que los dos casos se testean juntos.

// corrida separa stdout de stderr — CombinedOutput los mezcla y sería incapaz de ver este bug.
func corrida(t *testing.T, bin string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	cmd := exec.Command(bin, args...)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	code := 0
	var exitErr *exec.ExitError
	if err != nil {
		if !errors.As(err, &exitErr) {
			t.Fatalf("no se pudo ejecutar %s %v: %v", bin, args, err)
		}
		code = exitErr.ExitCode()
	}
	return outBuf.String(), errBuf.String(), code
}

func TestPrintHelp_FlagInvalido_ElUsageVaAStderrYNoTocaStdout(t *testing.T) {
	bin := compilarConVersion(t, "1.1.0")

	stdout, stderr, code := corrida(t, bin, "--flag-que-no-existe")

	if strings.TrimSpace(stdout) != "" {
		t.Errorf("el usage de un flag inválido contamina el stdout que parsea el hook SessionStart; stdout devolvió %q", stdout)
	}
	if !strings.Contains(stderr, "Uso:") {
		t.Errorf("el usage tiene que seguir llegando al usuario por stderr; stderr devolvió %q", stderr)
	}
	if code == 0 {
		t.Error("un flag inválido es un error de uso: el exit code no puede ser 0")
	}
}

func TestPrintHelp_PedidoExplicito_VaAStdoutYSaleCero(t *testing.T) {
	bin := compilarConVersion(t, "1.1.0")

	for _, flagDeAyuda := range []string{"--help", "-help", "-h"} {
		stdout, stderr, code := corrida(t, bin, flagDeAyuda)

		if !strings.Contains(stdout, "Uso:") {
			t.Errorf("%s: la ayuda pedida por el usuario es la salida legítima del programa y va a stdout; stdout devolvió %q", flagDeAyuda, stdout)
		}
		if strings.TrimSpace(stderr) != "" {
			t.Errorf("%s: pedir ayuda no es un error, stderr tiene que quedar limpio; devolvió %q", flagDeAyuda, stderr)
		}
		if code != 0 {
			t.Errorf("%s: pedir ayuda no es un error, el exit code tiene que ser 0; fue %d", flagDeAyuda, code)
		}
	}
}
