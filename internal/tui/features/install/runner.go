// Bridge entre la TUI install feature y la logica real de install
// (cmd/domain/install_cli.go). Esta capa existe para que la TUI
// sea testable sin ejecutar install real.
//
// En runtime, runInstallWithFlags() llama a la logica de
// install_cli.go. En tests, se mockea via SetInstallRunner().

package install

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// InstallRunner es la funcion que corre el install real.
// Tests lo mockean con SetInstallRunner.
var installRunner = defaultInstallRunner

// SetInstallRunner reemplaza el runner (solo para tests).
func SetInstallRunner(fn func(ctx context.Context, flags []string) error) {
	installRunner = fn
}

// runInstallWithFlags corre el install con los flags dados.
// Por default: invoca el binario `domain install` (sub-process)
// con los flags. Esto evita acoplar la TUI con la CLI main package
// (import ciclico) y mantiene la TUI standalone.
func runInstallWithFlags(ctx context.Context, flags []string) error {
	return installRunner(ctx, flags)
}

func defaultInstallRunner(ctx context.Context, flags []string) error {
	// Buscar el binario `domain` en $PATH. Si no esta, retornar
	// un error claro (estamos en dev: el binario no esta instalado).
	bin, err := exec.LookPath("domain")
	if err != nil {
		// Fallback: buscar ./bin/domain (convencion del repo).
		if _, statErr := os.Stat("./bin/domain"); statErr == nil {
			bin, _ = filepath.Abs("./bin/domain")
		} else {
			return fmt.Errorf("domain binary not found in PATH or ./bin/: %w", err)
		}
	}
	args := append([]string{"install"}, flags...)
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
