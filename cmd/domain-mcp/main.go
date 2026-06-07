// Package main es el entrypoint de `domain-mcp`: servidor MCP stdio.
//
// HU-12.1 mcp-core-stdio. Resuelve principal vía env var DOMAIN_API_KEY al
// boot y expone tools `domain_mem_save`, `domain_mem_search`,
// `domain_mem_context`, `domain_mem_get_observation` sobre stdin/stdout JSON-RPC.
//
// Variables de entorno:
//
//	DOMAIN_API_KEY            (requerido) — API key plaintext del user
//	DOMAIN_DATABASE_URL       (requerido) — DSN app_user pool
//	DOMAIN_DATABASE_AUTH_URL  (opcional)  — DSN app_admin pool; fallback al primero
//
// El proceso es one-shot por sesión MCP (un proceso por cliente conectado).
package main

import (
	"context"
	"fmt"
	"os"

	mcpgo "github.com/mark3labs/mcp-go/server"

	"github.com/saargo/domain/internal/audit"
	"github.com/saargo/domain/internal/auth/apikey"
	"github.com/saargo/domain/internal/config"
	"github.com/saargo/domain/internal/db"
	"github.com/saargo/domain/internal/llm"
	mcpserver "github.com/saargo/domain/internal/mcp/server"
	"github.com/saargo/domain/internal/service/observation"
	projsvc "github.com/saargo/domain/internal/service/project"
	promptsvc "github.com/saargo/domain/internal/service/prompt"
	searchsvc "github.com/saargo/domain/internal/service/search"
	sesssvc "github.com/saargo/domain/internal/service/session"
	timelinesvc "github.com/saargo/domain/internal/service/timeline"
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
			os.Exit(0)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "domain-mcp: config: %v\n", err)
		os.Exit(1)
	}
	apiKey := os.Getenv("DOMAIN_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "domain-mcp: DOMAIN_API_KEY requerido")
		os.Exit(1)
	}

	ctx := context.Background()
	pools, err := db.OpenProduction(ctx, cfg.DatabaseURL, cfg.DatabaseAuthURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "domain-mcp: pools: %v\n", err)
		os.Exit(1)
	}
	defer pools.Close()

	keys := &apikey.PGStore{Pool: pools.Auth}
	principal, err := keys.Resolve(ctx, apiKey)
	if err != nil {
		fmt.Fprintln(os.Stderr, "domain-mcp: API key inválida o revocada")
		os.Exit(1)
	}

	recorder := &audit.PGRecorder{Pool: pools.Auth}
	projects := &projsvc.Service{Pool: pools.App, Audit: recorder}
	observations := &observation.Service{
		Pool: pools.App, Audit: recorder, Embedder: llm.NopEmbedder{},
	}
	sessions := &sesssvc.Service{Pool: pools.App, Audit: recorder}
	prompts := &promptsvc.Service{Pool: pools.App, Audit: recorder}
	timeline := &timelinesvc.Service{Pool: pools.App}
	search := &searchsvc.Service{Pool: pools.App}

	srv := mcpserver.New(mcpserver.Deps{
		Observations: observations,
		Projects:     projects,
		Sessions:     sessions,
		Prompts:      prompts,
		Timeline:     timeline,
		Search:       search,
		Principal:    principal,
		ServerName:   "domain-mcp",
		ServerVer:    Version,
	})

	fmt.Fprintf(os.Stderr, "domain-mcp %s ready (org=%s user=%s)\n",
		Version, principal.OrganizationID, principal.UserID)

	if err := mcpgo.ServeStdio(srv); err != nil {
		fmt.Fprintf(os.Stderr, "domain-mcp: serve: %v\n", err)
		os.Exit(1)
	}
}
