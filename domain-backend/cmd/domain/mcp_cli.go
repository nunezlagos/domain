// Subcomandos `domain mcp {list,install,uninstall}` — issue F1 catálogo.
//
//   domain mcp list
//     Lista los mcp_providers disponibles (built-ins sembrados).
//
//   domain mcp install <provider> [--agent=opencode|claude-code|claude-desktop]
//                                [--config=<path>]
//     Instala el provider en el config del cliente especificado.
//     Idempotente. Backup del config previo con timestamp.
//
//   domain mcp uninstall <provider> [--agent=...] [--config=<path>]
//     Quita el provider del config.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/mcpinstaller"
)

func runMCP(ctx context.Context, args []string) int {
	if len(args) == 0 {
		printMCPHelp()
		return 1
	}
	switch args[0] {
	case "list":
		return runMCPList(ctx, args[1:])
	case "install":
		return runMCPInstall(ctx, args[1:])
	case "uninstall":
		return runMCPUninstall(ctx, args[1:])
	case "--help", "-h", "help":
		printMCPHelp()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "domain mcp %s: subcomando desconocido\n", args[0])
		printMCPHelp()
		return 1
	}
}

func printMCPHelp() {
	fmt.Println(`domain mcp — catálogo de MCPs instalables (issue F1)

Uso:
  domain mcp list
  domain mcp install <provider> [--agent=<opencode|claude-code|claude-desktop>] [--config=<path>]
  domain mcp uninstall <provider> [--agent=<...>] [--config=<path>]

Proveedores built-in: filesystem, fetch, git, github, memory, time.

Agentes soportados:
  opencode        opencode.json del proyecto actual
  claude-code     .mcp.json del proyecto actual
  claude-desktop  claude_desktop_config.json del usuario

Si --config no se pasa, se usa el path canónico según el agente.`)
}

func defaultConfigPath(agent mcpinstaller.Agent) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch agent {
	case mcpinstaller.AgentOpenCode:
		return filepath.Join(home, ".config", "opencode", "opencode.json"), nil
	case mcpinstaller.AgentClaudeCode:
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, ".mcp.json"), nil
	case mcpinstaller.AgentClaudeDesktop:
		return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json"), nil
	default:
		return "", fmt.Errorf("agente no soportado: %s", agent)
	}
}

type mcpFlags struct {
	agent  mcpinstaller.Agent
	config string
}

func parseMCPFlags(args []string) (mcpFlags, error) {
	f := mcpFlags{agent: mcpinstaller.AgentOpenCode}
	for i := 0; i < len(args); i++ {
		a := args[i]
		var key, val string
		hasVal := false
		if eq := indexEq(a); eq >= 0 {
			key, val, hasVal = a[:eq], a[eq+1:], true
		} else {
			key = a
		}
		switch key {
		case "--agent":
			if !hasVal {
				if i+1 >= len(args) {
					return f, fmt.Errorf("missing value for --agent")
				}
				val = args[i+1]
				i++
			}
			f.agent = mcpinstaller.Agent(val)
		case "--config":
			if !hasVal {
				if i+1 >= len(args) {
					return f, fmt.Errorf("missing value for --config")
				}
				val = args[i+1]
				i++
			}
			f.config = val
		default:
			return f, fmt.Errorf("unknown flag: %s", a)
		}
	}
	return f, nil
}

func indexEq(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return i
		}
	}
	return -1
}

func runMCPList(ctx context.Context, _ []string) int {
	pool, err := openPool()
	if err != nil {
		fmt.Fprintln(os.Stderr, "db:", err)
		return 1
	}
	defer pool.Close()

	svc := mcpinstaller.New(pool)
	providers, err := svc.List(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "list:", err)
		return 1
	}

	out, _ := json.MarshalIndent(map[string]any{
		"providers": providers,
		"total":     len(providers),
	}, "", "  ")
	fmt.Println(string(out))
	return 0
}

func runMCPInstall(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "install: provider name required")
		return 1
	}
	provider := args[0]
	flags, err := parseMCPFlags(args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "flags:", err)
		return 1
	}

	if flags.config == "" {
		flags.config, err = defaultConfigPath(flags.agent)
		if err != nil {
			fmt.Fprintln(os.Stderr, "config path:", err)
			return 1
		}
	}

	pool, err := openPool()
	if err != nil {
		fmt.Fprintln(os.Stderr, "db:", err)
		return 1
	}
	defer pool.Close()

	svc := mcpinstaller.New(pool)
	res, err := svc.Install(ctx, mcpinstaller.InstallInput{
		Provider:   provider,
		Agent:      flags.agent,
		ConfigPath: flags.config,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "install:", err)
		return 1
	}

	out, _ := json.MarshalIndent(res, "", "  ")
	fmt.Println(string(out))
	return 0
}

func runMCPUninstall(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uninstall: provider name required")
		return 1
	}
	provider := args[0]
	flags, err := parseMCPFlags(args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "flags:", err)
		return 1
	}

	if flags.config == "" {
		flags.config, err = defaultConfigPath(flags.agent)
		if err != nil {
			fmt.Fprintln(os.Stderr, "config path:", err)
			return 1
		}
	}

	pool, err := openPool()
	if err != nil {
		fmt.Fprintln(os.Stderr, "db:", err)
		return 1
	}
	defer pool.Close()

	svc := mcpinstaller.New(pool)
	if err := svc.Uninstall(ctx, mcpinstaller.UninstallInput{
		Provider:   provider,
		Agent:      flags.agent,
		ConfigPath: flags.config,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "uninstall:", err)
		return 1
	}
	fmt.Printf("uninstalled %s from %s\n", provider, flags.config)
	return 0
}

// openPool abre pool a la DB usando la config del env (.env cascade).
func openPool() (*pgxpool.Pool, error) {
	dsn := envOr("DOMAIN_DATABASE_URL", "postgres://domain:domain@localhost:5432/domain?sslmode=disable")
	return pgxpool.New(context.Background(), dsn)
}
