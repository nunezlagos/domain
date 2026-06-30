package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nunezlagos/domain/internal/cli/install"
)

// checkProjectRootGuard valida que el cwd (o --src) sea un root
// válido del repo domain. Si no, aborta con exit 1 y mensaje
// accionable. Si --src se pasa, hace os.Chdir al path absoluto.
//
// Retorna (projectRoot, ok). Si ok=false, runInstall debe
// retornar 1 con el mensaje ya impreso.
//
// Esta función se invoca como PRIMER step de runInstall, ANTES
// de cualquier side effect (backups, loadEnv, docker, etc).
func checkProjectRootGuard(srcFlag string) (string, bool) {
	projectRoot := srcFlag
	if projectRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: could not determine cwd: %v\n", err)
			return "", false
		}
		projectRoot = cwd
	}

	ok, missing, err := install.IsProjectRoot(projectRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: project root check failed for %q: %v\n", projectRoot, err)
		return projectRoot, false
	}
	if !ok {
		fmt.Fprintf(os.Stderr,
			"no estás en el root del repo domain (cwd=%s, missing files: %s). "+
				"Corré `bash install.sh` o pasá --src /path/al/repo\n",
			projectRoot, strings.Join(missing, ", "))
		return projectRoot, false
	}

	if srcFlag != "" {
		absSrc, err := filepath.Abs(srcFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: could not resolve --src %q: %v\n", srcFlag, err)
			return projectRoot, false
		}
		if err := os.Chdir(absSrc); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: chdir to %s: %v\n", absSrc, err)
			return projectRoot, false
		}
		projectRoot = absSrc
	}

	return projectRoot, true
}
