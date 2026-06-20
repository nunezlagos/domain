package httpserver

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestListenAndServeWithFatalLog_PortBusy verifica que cuando el
// puerto está ocupado, la función loggea "FATAL: HTTP listener
// failed" y retorna el error.
//
// Si se comenta el `logger.Error(...)` o se omite el return err
// (sabotaje), este test DEBE FALLAR.
func TestListenAndServeWithFatalLog_PortBusy(t *testing.T) {
	// Ocupar el puerto: net.Listen sostiene el puerto hasta close.
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer blocker.Close()
	port := blocker.Addr().(*net.TCPAddr).Port

	// Capturar log en un buffer
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Segundo srv intenta el mismo puerto
	srv := &http.Server{Addr: net.JoinHostPort("127.0.0.1", itoa(port)), Handler: http.NewServeMux()}

	// Correr la función en goroutine para no bloquear
	done := make(chan error, 1)
	go func() {
		done <- ListenAndServeWithFatalLog(srv, logger)
	}()

	select {
	case err := <-done:
		require.Error(t, err, "ListenAndServeWithFatalLog debe retornar error cuando el puerto está ocupado")
		require.Contains(t, logBuf.String(), "FATAL: HTTP listener failed",
			"log debe contener 'FATAL: HTTP listener failed', got: %s", logBuf.String())
	case <-time.After(3 * time.Second):
		t.Fatal("ListenAndServeWithFatalLog no retornó en 3s")
	}
}

// TestListenAndServeWithFatalLog_NilLogger no debe crashear aunque
// logger sea nil (defensivo).
func TestListenAndServeWithFatalLog_NilLogger(t *testing.T) {
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer blocker.Close()
	port := blocker.Addr().(*net.TCPAddr).Port

	srv := &http.Server{Addr: net.JoinHostPort("127.0.0.1", itoa(port)), Handler: http.NewServeMux()}

	done := make(chan error, 1)
	go func() {
		done <- ListenAndServeWithFatalLog(srv, nil)
	}()

	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("no retornó en 3s")
	}
}

// TestRunPostBindWatchdog_FatalCalled verifica que el watchdog
// llama a fatalFn si los 3 intentos de ProbeHealth fallan.
func TestRunPostBindWatchdog_FatalCalled(t *testing.T) {
	// Port 1: privileged y casi siempre cerrado.
	var fatalCalled bool
	var fatalErr error
	var mu sync.Mutex
	fatalFn := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		fatalCalled = true
		fatalErr = err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := time.Now()
	RunPostBindWatchdog(ctx, 1, fatalFn)
	elapsed := time.Since(start)

	// 2s settle + 3 * 1s intentos = ~5s
	require.GreaterOrEqual(t, elapsed, 4*time.Second,
		"watchdog debe esperar al menos 4s antes de fatal (settle 2s + 3 retries de 1s)")

	mu.Lock()
	defer mu.Unlock()
	require.True(t, fatalCalled, "fatalFn debe ser llamado cuando ProbeHealth falla 3x")
	require.Error(t, fatalErr, "fatalFn debe recibir el último error")
}

// TestRunPostBindWatchdog_ContextCancel verifica que el watchdog
// respeta el context (no bloquea para siempre).
func TestRunPostBindWatchdog_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelado inmediatamente

	fatalCalled := false
	RunPostBindWatchdog(ctx, 80, func(error) { fatalCalled = true })
	require.False(t, fatalCalled, "fatalFn no debe ser llamado si el context ya está cancelado")
}

// TestRunPostBindWatchdog_NilFatalFn no debe crashear.
func TestRunPostBindWatchdog_NilFatalFn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// No debe crashear
	RunPostBindWatchdog(ctx, 1, nil)
}

// itoa helper para evitar strconv en el test file (mantener corto).
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
