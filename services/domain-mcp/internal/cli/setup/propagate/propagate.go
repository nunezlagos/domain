package propagate

import (
	"fmt"
	"os/exec"
)

// execCommand existe como punto de inyección para los tests. Propagate invoca el
// binario `domain` del PATH, así que su contrato —seguir con los demás proyectos
// cuando uno falla— solo se podía verificar en máquinas donde ese binario NO está
// instalado: donde SÍ lo está, `domain setup auto-detect` devuelve exit 0 incluso
// con un path inexistente y el test veía éxitos donde esperaba fallos.
var execCommand = exec.Command

func Propagate(selected []ProjectInfo, dryRun bool) (success, failed int, errs []error) {
	if dryRun {
		return 0, 0, nil
	}

	for _, info := range selected {
		cmd := execCommand("domain", "setup", "auto-detect", info.Path, "--quiet")
		output, err := cmd.CombinedOutput()
		if err != nil {
			failed++
			errs = append(errs, fmt.Errorf("%s: %w\n%s", info.Name, err, string(output)))
		} else {
			success++
		}
	}
	return
}
