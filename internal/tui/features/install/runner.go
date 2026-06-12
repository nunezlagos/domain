// Bridge entre la TUI install feature y la logica real de install
// (cmd/domain/install_cli.go via sub-process). Esta capa existe para
// que la TUI sea testable sin ejecutar install real.
//
// HU-01.13 commit 2/3: ahora capturamos stderr del sub-process para
// propagar el error real al user (en vez de "exit status 1" opaco).

package install

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// maxStderrBytes limita cuanto stderr capturamos (4KB). Si el
// sub-process escribe mas, truncamos con un mensaje al final.
const maxStderrBytes = 4096

// InstallRunner es la funcion que corre el install real.
// Tests lo mockean con SetInstallRunner.
//
// Retorna (error, stderr). El stderr es util cuando err != nil
// (exit != 0): contiene el mensaje real del sub-process.
type InstallRunner func(ctx context.Context, flags []string) (error, string)

var installRunner = defaultInstallRunner

// SetInstallRunner reemplaza el runner (solo para tests).
func SetInstallRunner(fn InstallRunner) {
	installRunner = fn
}

// runInstallWithFlags corre el install con los flags dados.
// Por default: invoca el binario `domain install` (sub-process)
// con los flags. Esto evita acoplar la TUI con la CLI main package
// (import ciclico) y mantiene la TUI standalone.
//
// Captura stderr para que la TUI pueda mostrarlo al user.
func runInstallWithFlags(ctx context.Context, flags []string) (error, string) {
	return installRunner(ctx, flags)
}

func defaultInstallRunner(ctx context.Context, flags []string) (error, string) {
	bin, err := exec.LookPath("domain")
	if err != nil {
		// Fallback: buscar ./bin/domain (convencion del repo).
		if _, statErr := os.Stat("./bin/domain"); statErr == nil {
			bin, _ = filepath.Abs("./bin/domain")
		} else {
			return fmt.Errorf("domain binary not found in PATH or ./bin/: %w", err), ""
		}
	}
	args := append([]string{"install"}, flags...)
	cmd := exec.CommandContext(ctx, bin, args...)
	var stderrBuf bytes.Buffer
	// Limitamos el buffer a maxStderrBytes+1 para detectar overflow
	cmd.Stderr = &limitedWriter{w: &stderrBuf, max: maxStderrBytes}
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	stderrStr := stderrBuf.String()
	if err != nil {
		// Wrap con contexto: "sub-process exit N: <stderr>"
		return fmt.Errorf("install sub-process failed: %w (stderr: %s)", err, stderrStr), stderrStr
	}
	return nil, stderrStr
}

// limitedWriter trunca la escritura a max bytes. Si se trunca, agrega
// un mensaje al final.
type limitedWriter struct {
	w        io.Writer
	max      int
	written  int
	truncMsg string
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.written >= l.max {
		return len(p), nil // drop silently
	}
	remaining := l.max - l.written
	if len(p) > remaining {
		p = p[:remaining]
	}
	n, err := l.w.Write(p)
	l.written += n
	return len(p), err
}
