// Package main es el entrypoint del binario `domain`: CLI principal + servidor HTTP.
//
// HU-01.3 health-version (skeleton) + HU-14.1 cli-core-commands (skeleton).
// Esta implementación de Fase 0 solo provee `version` para validar build.
// El resto de comandos se implementa en Fase 1+.
package main

import (
	"fmt"
	"os"
)

// Variables sobrescritas por `-ldflags "-X main.Version=..."` (HU-19.2).
var (
	Version   = "0.0.0-dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Printf("domain %s\ncommit: %s\nbuilt: %s\n", Version, Commit, BuildTime)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "comando no implementado en Fase 0: %s\n", os.Args[1])
		fmt.Fprintln(os.Stderr, "ver `make help` y openspec/changes/REQ-14-cli/")
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Println(`domain — plataforma de memoria y orquestación para agentes AI

Uso:
  domain <comando> [args]

Comandos:
  version    Muestra version + commit + build time
  help       Muestra esta ayuda

Más comandos vienen en Fase 1+ (REQ-14 cli, openspec/INDEX.md).`)
}
