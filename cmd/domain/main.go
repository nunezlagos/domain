// Package main es el entrypoint del binario `domain`: CLI principal + servidor HTTP.
//
// HU-01.1 db-schema-migrations: subcomandos `migrate up|down|version`.
// HU-01.3 health-version: subcomando `version` y `server`.
// HU-14.1 cli-core-commands: estructura base; subcomandos restantes en Fase 2+.
package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/saargo/domain/internal/config"
	dmigrate "github.com/saargo/domain/internal/migrate"
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
	case "migrate":
		runMigrate(os.Args[2:])
	case "server":
		runServer()
	case "healthcheck":
		runHealthcheckProbe()
	default:
		fmt.Fprintf(os.Stderr, "comando no implementado: %s\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Println(`domain — plataforma de memoria y orquestación para agentes AI

Uso:
  domain <comando> [args]

Comandos:
  version             Muestra version + commit + build time
  help                Muestra esta ayuda
  migrate up          Aplica todas las migraciones DB pendientes
  migrate down [N]    Rollback N migraciones (default 1)
  migrate version     Muestra version actual del schema + dirty flag
  server              Inicia servidor HTTP (HU-01.3 /health)
  healthcheck         Probe interno para Dockerfile HEALTHCHECK

Más comandos vienen en Fase 2+ (ver openspec/INDEX.md y docs/roadmap.md).`)
}

func runMigrate(args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: domain migrate <up|down [N]|version>")
		os.Exit(2)
	}
	switch args[0] {
	case "up":
		if err := dmigrate.Up(cfg.DatabaseURL); err != nil {
			fmt.Fprintf(os.Stderr, "migrate up: %v\n", err)
			os.Exit(1)
		}
		v, dirty, _ := dmigrate.Version(cfg.DatabaseURL)
		fmt.Printf("migrations applied. current version: %d (dirty=%v)\n", v, dirty)
	case "down":
		steps := 1
		if len(args) > 1 {
			n, err := strconv.Atoi(args[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "invalid N: %v\n", err)
				os.Exit(2)
			}
			steps = n
		}
		if err := dmigrate.Down(cfg.DatabaseURL, steps); err != nil {
			fmt.Fprintf(os.Stderr, "migrate down: %v\n", err)
			os.Exit(1)
		}
		v, dirty, _ := dmigrate.Version(cfg.DatabaseURL)
		fmt.Printf("migrations rolled back. current version: %d (dirty=%v)\n", v, dirty)
	case "version":
		v, dirty, err := dmigrate.Version(cfg.DatabaseURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "migrate version: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("schema version: %d (dirty=%v)\n", v, dirty)
	default:
		fmt.Fprintf(os.Stderr, "unknown migrate subcommand: %s\n", args[0])
		os.Exit(2)
	}
}

func runServer() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	addr := fmt.Sprintf("%s:%d", cfg.HTTPBind, cfg.HTTPPort)
	mux := http.NewServeMux()
	startedAt := time.Now()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		uptime := time.Since(startedAt).Round(time.Second).String()
		fmt.Fprintf(w, `{"status":"ok","version":"%s","commit":"%s","built":"%s","uptime":"%s"}`,
			Version, Commit, BuildTime, uptime)
	})
	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprintf(w, `{"ready":true}`)
	})
	fmt.Printf("domain %s server listening on %s\n", Version, addr)
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  time.Duration(cfg.HTTPReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.HTTPWriteTimeoutSeconds) * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "server: %v\n", err)
		os.Exit(1)
	}
}

func runHealthcheckProbe() {
	cfg, err := config.Load()
	if err != nil {
		os.Exit(1)
	}
	url := fmt.Sprintf("http://%s:%d/health", cfg.HTTPBind, cfg.HTTPPort)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url) //nolint:gosec // local probe
	if err != nil {
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		os.Exit(1)
	}
	os.Exit(0)
}
