package acp

import "time"

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
}

const (
	defaultBin     = "opencode"
	defaultTimeout = 120 * time.Second
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
	return c
}
