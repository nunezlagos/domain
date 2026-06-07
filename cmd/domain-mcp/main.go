// Package main es el entrypoint del binario `domain-mcp`: MCP server stdio.
//
// HU-12.1 mcp-core-stdio (skeleton).
// Esta implementación de Fase 0 solo provee `version`. El servidor MCP real se implementa en Fase 2.
package main

import (
	"fmt"
	"os"
)

var (
	Version   = "0.0.0-dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "--version", "-v":
			fmt.Printf("domain-mcp %s\ncommit: %s\nbuilt: %s\n", Version, Commit, BuildTime)
			return
		case "healthcheck":
			// stub para Dockerfile HEALTHCHECK (HU-19.3)
			os.Exit(0)
		}
	}
	fmt.Fprintln(os.Stderr, "domain-mcp: MCP server stdio no implementado todavía (Fase 2, REQ-12).")
	fmt.Fprintln(os.Stderr, "Use: domain-mcp version | healthcheck")
	os.Exit(2)
}
