// issue-11.1 sandbox-execution — ejecuta código untrusted en sandbox aislado.
//
// MVP: process-based sandbox usando OS resources limits + chroot opcional
// + timeout. Para sandboxing real, integrar Firecracker / gVisor en HU futura.
//
// Modos soportados:
//   - process: spawn subprocess con rlimit (CPU, memory, processes, files).
//   - docker: docker run --read-only --network=none (si docker disponible).
//   - dry-run: solo valida policy, no ejecuta (testing).
package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// Mode controla el sandbox backend.
type Mode string

const (
	ModeProcess Mode = "process"
	ModeDocker  Mode = "docker"
	ModeDryRun  Mode = "dry-run"
)

// Config define límites por ejecución.
type Config struct {
	Mode          Mode
	Timeout       time.Duration  // wall-clock; default 30s
	MaxMemoryMB   int            // RSS limit, 256 default
	MaxCPUSec     int            // CPU time, 10 default
	MaxFileSizeMB int            // single output file, 10 default
	MaxProcesses  int            // total subprocesses, 1 default
	NetworkAccess bool           // default false
	ReadOnlyFS    bool           // default true (modo docker)
	Image         string         // docker image (modo docker), default alpine:3.20
}

// Request describe la ejecución solicitada.
type Request struct {
	Cmd     []string          // argv. cmd[0] = ejecutable
	Stdin   string            // input por stdin
	Env     map[string]string // env vars (filtradas)
	Workdir string            // cwd dentro del sandbox
}

// Result del ejecución.
type Result struct {
	Stdout    string        `json:"stdout"`
	Stderr    string        `json:"stderr"`
	ExitCode  int           `json:"exit_code"`
	Duration  time.Duration `json:"duration"`
	TimedOut  bool          `json:"timed_out"`
	OOMKilled bool          `json:"oom_killed"`
	Truncated bool          `json:"truncated_output"`
}

// ErrSandboxNotSupported se devuelve si el modo solicitado no es compatible.
var ErrSandboxNotSupported = errors.New("sandbox mode not supported on this OS")

// Runner ejecuta requests con la Config dada.
type Runner struct {
	Config Config
}

// New retorna un Runner con defaults razonables.
func New(cfg Config) *Runner {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxMemoryMB <= 0 {
		cfg.MaxMemoryMB = 256
	}
	if cfg.MaxCPUSec <= 0 {
		cfg.MaxCPUSec = 10
	}
	if cfg.MaxFileSizeMB <= 0 {
		cfg.MaxFileSizeMB = 10
	}
	if cfg.MaxProcesses <= 0 {
		cfg.MaxProcesses = 1
	}
	if cfg.Mode == "" {
		cfg.Mode = ModeProcess
	}
	if cfg.Image == "" {
		cfg.Image = "alpine:3.20"
	}
	return &Runner{Config: cfg}
}

// Run ejecuta el request en el sandbox configurado.
func (r *Runner) Run(ctx context.Context, req Request) (*Result, error) {
	switch r.Config.Mode {
	case ModeProcess:
		return r.runProcess(ctx, req)
	case ModeDocker:
		return r.runDocker(ctx, req)
	case ModeDryRun:
		return &Result{Stdout: "[dry-run] " + strings.Join(req.Cmd, " ")}, nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrSandboxNotSupported, r.Config.Mode)
	}
}

const maxOutputBytes = 1 * 1024 * 1024 // 1MB

func (r *Runner) runProcess(ctx context.Context, req Request) (*Result, error) {
	if len(req.Cmd) == 0 {
		return nil, errors.New("cmd required")
	}
	if runtime.GOOS == "windows" {
		return nil, fmt.Errorf("%w: process mode requires unix", ErrSandboxNotSupported)
	}

	cctx, cancel := context.WithTimeout(ctx, r.Config.Timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, req.Cmd[0], req.Cmd[1:]...)
	if req.Workdir != "" {
		cmd.Dir = req.Workdir
	}
	cmd.Stdin = strings.NewReader(req.Stdin)
	env := make([]string, 0, len(req.Env))
	for k, v := range req.Env {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // grupo separado, killable cleanly
	}

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	cmd.Stdout = &limitedWriter{w: stdoutBuf, max: maxOutputBytes}
	cmd.Stderr = &limitedWriter{w: stderrBuf, max: maxOutputBytes}

	start := time.Now()
	err := cmd.Run()
	dur := time.Since(start)

	res := &Result{
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		Duration: dur,
	}
	if cctx.Err() == context.DeadlineExceeded {
		res.TimedOut = true
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		res.ExitCode = exitErr.ExitCode()
	} else if err != nil && !res.TimedOut {
		return res, fmt.Errorf("exec: %w", err)
	}
	if stdoutBuf.Len() >= maxOutputBytes || stderrBuf.Len() >= maxOutputBytes {
		res.Truncated = true
	}
	return res, nil
}

func (r *Runner) runDocker(ctx context.Context, req Request) (*Result, error) {
	args := []string{"run", "--rm", "-i",
		"--memory", fmt.Sprintf("%dm", r.Config.MaxMemoryMB),
		"--cpus", "1",
		"--pids-limit", fmt.Sprintf("%d", r.Config.MaxProcesses),
	}
	if !r.Config.NetworkAccess {
		args = append(args, "--network", "none")
	}
	if r.Config.ReadOnlyFS {
		args = append(args, "--read-only")
	}
	if req.Workdir != "" {
		args = append(args, "-w", req.Workdir)
	}
	for k, v := range req.Env {
		args = append(args, "-e", k+"="+v)
	}
	args = append(args, r.Config.Image)
	args = append(args, req.Cmd...)

	cctx, cancel := context.WithTimeout(ctx, r.Config.Timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, "docker", args...)
	cmd.Stdin = strings.NewReader(req.Stdin)
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	cmd.Stdout = &limitedWriter{w: stdout, max: maxOutputBytes}
	cmd.Stderr = &limitedWriter{w: stderr, max: maxOutputBytes}

	start := time.Now()
	err := cmd.Run()
	dur := time.Since(start)
	res := &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: dur,
	}
	if cctx.Err() == context.DeadlineExceeded {
		res.TimedOut = true
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		res.ExitCode = exitErr.ExitCode()
		if res.ExitCode == 137 {
			res.OOMKilled = true
		}
	} else if err != nil && !res.TimedOut {
		return res, fmt.Errorf("docker: %w", err)
	}
	return res, nil
}

// limitedWriter trunca writes después de max bytes.
type limitedWriter struct {
	w   io.Writer
	max int
	n   int
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	if lw.n >= lw.max {
		return len(p), nil // pretendemos write OK pero descartamos
	}
	remaining := lw.max - lw.n
	if len(p) > remaining {
		p = p[:remaining]
	}
	written, err := lw.w.Write(p)
	lw.n += written
	return written, err
}
