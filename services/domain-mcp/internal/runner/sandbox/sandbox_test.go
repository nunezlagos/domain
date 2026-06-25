package sandbox

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)






func TestNew_Defaults(t *testing.T) {
	r := New(Config{})
	require.Equal(t, ModeProcess, r.Config.Mode)
	require.Equal(t, 30*time.Second, r.Config.Timeout)
	require.Equal(t, 256, r.Config.MaxMemoryMB)
	require.Equal(t, 10, r.Config.MaxCPUSec)
	require.Equal(t, 10, r.Config.MaxFileSizeMB)
	require.Equal(t, 1, r.Config.MaxProcesses)
	require.Equal(t, "alpine:3.20", r.Config.Image)
}

func TestNew_PreservesCustomConfig(t *testing.T) {
	cfg := Config{
		Mode:        ModeDocker,
		Timeout:     5 * time.Second,
		MaxMemoryMB: 512,
		Image:       "python:3.12-slim",
	}
	r := New(cfg)
	require.Equal(t, ModeDocker, r.Config.Mode)
	require.Equal(t, 5*time.Second, r.Config.Timeout)
	require.Equal(t, 512, r.Config.MaxMemoryMB)
	require.Equal(t, "python:3.12-slim", r.Config.Image)
}

func TestRun_DryRun_DoesNotExecute(t *testing.T) {


	r := New(Config{Mode: ModeDryRun})
	res, err := r.Run(context.Background(), Request{
		Cmd: []string{"echo", "hello", "world"},
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, "[dry-run] echo hello world", res.Stdout)
	require.Empty(t, res.Stderr)
	require.Equal(t, 0, res.ExitCode)
}

func TestRun_UnsupportedMode(t *testing.T) {

	r := New(Config{Mode: "weird-mode"})
	_, err := r.Run(context.Background(), Request{Cmd: []string{"true"}})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrSandboxNotSupported))
}

func TestRun_ProcessMode_EmptyCmd(t *testing.T) {
	r := New(Config{Mode: ModeProcess})
	_, err := r.Run(context.Background(), Request{Cmd: nil})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cmd required")
}

func TestRun_ProcessMode_Echo(t *testing.T) {



	if testing.Short() {
		t.Skip("skip en short mode (spawnea proceso)")
	}
	r := New(Config{Mode: ModeProcess, Timeout: 5 * time.Second})
	res, err := r.Run(context.Background(), Request{
		Cmd: []string{"echo", "hello"},
	})
	require.NoError(t, err)
	require.Equal(t, 0, res.ExitCode)
	require.Equal(t, "hello\n", res.Stdout)
	require.Empty(t, res.Stderr)
	require.False(t, res.TimedOut)
}

func TestRun_ProcessMode_StdinPassthrough(t *testing.T) {
	if testing.Short() {
		t.Skip("skip en short mode")
	}


	r := New(Config{Mode: ModeProcess, Timeout: 5 * time.Second})
	res, err := r.Run(context.Background(), Request{
		Cmd:   []string{"cat"},
		Stdin: "input-from-test",
	})
	require.NoError(t, err)
	require.Equal(t, 0, res.ExitCode)
	require.Equal(t, "input-from-test", res.Stdout)
}

func TestRun_ProcessMode_NonZeroExit(t *testing.T) {
	if testing.Short() {
		t.Skip("skip en short mode")
	}

	r := New(Config{Mode: ModeProcess, Timeout: 5 * time.Second})
	res, err := r.Run(context.Background(), Request{
		Cmd: []string{"false"},
	})
	require.NoError(t, err, "ExitError no es error de runtime, es parte del resultado")
	require.Equal(t, 1, res.ExitCode)
}

func TestRun_ProcessMode_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skip en short mode (sleep 2s)")
	}

	r := New(Config{Mode: ModeProcess, Timeout: 100 * time.Millisecond})
	res, err := r.Run(context.Background(), Request{
		Cmd: []string{"sleep", "2"},
	})
	require.NoError(t, err)
	require.True(t, res.TimedOut, "sleep 2 con timeout 100ms DEBE marcar TimedOut=true")
}

func TestRun_ProcessMode_ContextCancel(t *testing.T) {
	if testing.Short() {
		t.Skip("skip en short mode")
	}










	r := New(Config{Mode: ModeProcess, Timeout: 10 * time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	res, err := r.Run(ctx, Request{
		Cmd: []string{"sleep", "5"},
	})
	require.NoError(t, err)


	require.NotEqual(t, 0, res.ExitCode, "subproceso matado por ctx cancel → exit code != 0")
	_ = res.TimedOut // intencionalmente no assertamos: ver nota arriba
}

func TestRun_ProcessMode_OutputTruncation(t *testing.T) {
	if testing.Short() {
		t.Skip("skip en short mode (genera 2MB output)")
	}

	r := New(Config{Mode: ModeProcess, Timeout: 5 * time.Second})
	res, err := r.Run(context.Background(), Request{
		Cmd: []string{"sh", "-c", "head -c 2097152 /dev/urandom | base64"},
	})
	require.NoError(t, err)
	require.True(t, res.Truncated, "output > 1MB DEBE marcar Truncated=true")
	require.LessOrEqual(t, len(res.Stdout), maxOutputBytes+100, "stdout <= 1MB")
}

func TestLimitedWriter_Truncates(t *testing.T) {
	var buf strings.Builder
	lw := &limitedWriter{w: &buf, max: 10}

	n, err := lw.Write([]byte("hello"))
	require.NoError(t, err)
	require.Equal(t, 5, n)

	n, err = lw.Write([]byte("world"))
	require.NoError(t, err)
	require.Equal(t, 5, n, "segundo write cabe exacto (10-5=5)")





	n, err = lw.Write([]byte("extra"))
	require.NoError(t, err)
	require.Equal(t, 5, n, "5 bytes 'consumidos' del input (aunque descartados)")

	require.Equal(t, "helloworld", buf.String(), "buffer solo tiene los primeros 10 bytes")
}

func TestLimitedWriter_ZeroMax_DropsAll(t *testing.T) {
	var buf strings.Builder
	lw := &limitedWriter{w: &buf, max: 0}

	n, err := lw.Write([]byte("anything"))
	require.NoError(t, err)
	require.Equal(t, 8, n, "8 bytes 'consumidos' aunque descartados")
	require.Empty(t, buf.String())
}

func TestRun_ProcessMode_WorkdirAndEnv(t *testing.T) {
	if testing.Short() {
		t.Skip("skip en short mode")
	}


	r := New(Config{Mode: ModeProcess, Timeout: 5 * time.Second})
	res, err := r.Run(context.Background(), Request{
		Cmd:     []string{"sh", "-c", "pwd && echo $DOMAIN_TEST_ENV"},
		Workdir: "/tmp",
		Env:     map[string]string{"DOMAIN_TEST_ENV": "hello-from-test"},
	})
	require.NoError(t, err)
	require.Equal(t, 0, res.ExitCode)
	require.Contains(t, res.Stdout, "/tmp", "workdir se aplica")
	require.Contains(t, res.Stdout, "hello-from-test", "env var se aplica")
}
