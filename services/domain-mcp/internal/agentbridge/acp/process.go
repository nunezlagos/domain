package acp

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Process es un Session respaldado por un subproceso `opencode acp`. Modela el
// spawn+lifecycle sobre el patrón de internal/mcp/client/stdio.go
type Process struct {
	*Session
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	closeOnce sync.Once
}

// Spawn lanza el agente como subproceso y devuelve un Process listo para Prompt.
// Degrada con error si el binario no existe (no crashea el server)
func Spawn(ctx context.Context, cfg Config, logger *slog.Logger) (*Process, error) {
	cfg = cfg.withDefaults()
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Cwd == "" {
		if wd, err := os.Getwd(); err == nil {
			cfg.Cwd = wd
		}
	}

	procCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(procCtx, cfg.Bin, cfg.Args...)
	cmd.Env = append(os.Environ(), cfg.Env...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("acp stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("acp stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("acp start %s: %w", cfg.Bin, err)
	}
	logger.Info("acp subprocess started", slog.String("bin", cfg.Bin), slog.String("cwd", cfg.Cwd))

	return &Process{
		Session: newSession(stdin, stdout, cfg.Cwd),
		cmd:     cmd,
		cancel:  cancel,
	}, nil
}

// Close termina el subproceso: cancela el ctx (dispara el kill de CommandContext),
// espera con timeout y mata si no salió. Idempotente
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
	})
	return err
}
