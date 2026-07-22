package acp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
)

// Process es un Session respaldado por un subproceso `opencode acp` (spawn +
// lifecycle sobre el patrón de internal/mcp/client/stdio.go).
type Process struct {
	*Session
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	closeOnce sync.Once
	ws        *Workspace // workspace del run; se limpia en Close si Spawn lo creó
}

// Spawn lanza el agente como subproceso y devuelve un Process listo para Prompt.
// Degrada con error si el binario no existe. McpURL set → sesión nativa.
func Spawn(ctx context.Context, cfg Config, logger *slog.Logger) (*Process, error) {
	cfg = cfg.withDefaults()
	if logger == nil {
		logger = slog.Default()
	}
	ws, err := prepareWorkspace(&cfg)
	if err != nil {
		return nil, err
	}
	// Cleanup condicional: sin esto un fallo tras crear el workspace dejaría el
	// dir temp huérfano. Se desarma (success=true) al devolver el *Process; de
	// ahí el ownership del cleanup pasa a Process.Close.
	success := false
	defer func() {
		if !success && ws != nil {
			_ = ws.Cleanup()
		}
	}()

	procCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(procCtx, cfg.Bin, cfg.Args...)
	cmd.Env = scrubbedEnv(cfg.Env) // allowlist explícito: sin secretos del server

	stdin, stdout, err := pipes(cmd)
	if err != nil {
		cancel()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("acp start %s: %w", cfg.Bin, err)
	}
	logger.Info("acp subprocess started", slog.Any("cfg", cfg))

	var mcp *acpsdk.McpServer
	if cfg.McpURL != "" {
		mcp = buildMcpServer(cfg)
	}
	h := &handler{ws: ws, permissionMode: cfg.PermissionMode}
	p := &Process{
		Session: newSessionWithHandler(stdin, stdout, cfg.Cwd, h, mcp),
		cmd:     cmd,
		cancel:  cancel,
		ws:      ws,
	}
	success = true
	return p, nil
}

func pipes(cmd *exec.Cmd) (io.WriteCloser, io.ReadCloser, error) {
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("acp stdin: %w", err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("acp stdout: %w", err)
	}
	return in, out, nil
}

// prepareWorkspace crea el workspace del run en modo nativo (McpURL set) y fija
// cfg.Cwd al root. Núcleo liviano (McpURL vacío) no crea workspace y respeta el
// cwd provisto (o el del server como fallback).
func prepareWorkspace(cfg *Config) (*Workspace, error) {
	if cfg.McpURL == "" {
		if cfg.Cwd == "" {
			if wd, err := os.Getwd(); err == nil {
				cfg.Cwd = wd
			}
		}
		return nil, nil
	}
	ws, err := makeWorkspace(cfg.WorkspaceRoot)
	if err != nil {
		return nil, err
	}
	cfg.Cwd = ws.Root()
	return ws, nil
}

func makeWorkspace(root string) (*Workspace, error) {
	if root != "" {
		return openWorkspace(root)
	}
	return NewWorkspace()
}

// scrubbedEnv arma el env del subproceso como allowlist EXPLÍCITO: NO hereda
// os.Environ(). Solo pasan variables de tooling neutro; los secretos del server
// NUNCA llegan al agente. extra (cfg.Env) agrega lo que opencode necesite.
func scrubbedEnv(extra []string) []string {
	allow := []string{"PATH", "HOME", "LANG", "LC_ALL", "LC_CTYPE", "TMPDIR", "TERM", "USER", "SHELL"}
	var env []string
	for _, k := range allow {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return append(env, extra...)
}

// Close termina el subproceso: cancela el ctx, espera con timeout y mata si no
// salió. Limpia el workspace. Idempotente.
func (p *Process) Close() error {
	var err error
	p.closeOnce.Do(func() {
		p.cancel()
		done := make(chan error, 1)
		go func() { done <- p.cmd.Wait() }()
		select {
		case err = <-done:
		case <-time.After(2 * time.Second):
			_ = p.cmd.Process.Kill()
			err = <-done
		}
		if p.ws != nil {
			_ = p.ws.Cleanup()
		}
	})
	return err
}
