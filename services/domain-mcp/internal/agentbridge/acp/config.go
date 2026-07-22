package acp

import (
	"log/slog"
	"time"
)

// Config parametriza el bridge ACP hacia opencode
type Config struct {
	// Bin es el binario del agente (default "opencode")
	Bin string
	// Args son los argumentos del agente (default ["acp"])
	Args []string
	// Cwd es el workspace de la sesión; debe ser absoluto
	Cwd string
	// Timeout acota un turno de prompt (default 120s)
	Timeout time.Duration
	// Env agrega variables KEY=VALUE al subproceso; nunca se loguea
	Env []string
	// WorkspaceRoot acota las operaciones de fs del agente (per-run temp dir)
	WorkspaceRoot string
	// McpURL es el endpoint del MCP que se expone al agente; vacío = núcleo liviano
	McpURL string
	// McpToken es el bearer con que el agente autentica contra el MCP; nunca se loguea
	McpToken string
	// PermissionMode gobierna writes/permisos del agente (default "deny-all")
	PermissionMode string
}

const (
	defaultBin     = "opencode"
	defaultTimeout = 120 * time.Second
	// PermissionDenyAll es el modo seguro por default: no writes, permisos denegados
	PermissionDenyAll = "deny-all"
)

func (c Config) withDefaults() Config {
	if c.Bin == "" {
		c.Bin = defaultBin
	}
	if len(c.Args) == 0 {
		c.Args = []string{"acp"}
	}
	if c.Timeout == 0 {
		c.Timeout = defaultTimeout
	}
	if c.PermissionMode == "" {
		c.PermissionMode = PermissionDenyAll
	}
	return c
}

// LogValue implementa slog.LogValuer: redacta el token y el env (portadores de
// secretos) para que nunca aparezcan en los logs estructurados.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("bin", c.Bin),
		slog.String("cwd", c.Cwd),
		slog.Duration("timeout", c.Timeout),
		slog.String("mcp_url", c.McpURL),
		slog.String("mcp_token", redacted(c.McpToken)),
		slog.String("permission_mode", c.PermissionMode),
		slog.Int("env_vars", len(c.Env)),
	)
}

// redacted enmascara un secreto: "" si vacío, "***" si presente
func redacted(s string) string {
	if s == "" {
		return ""
	}
	return "***"
}
