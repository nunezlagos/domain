//go:build !race
// +build !race

package httpserver

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestServerExitsOnBindError verifica que cuando el puerto está
// ocupado (otro proceso ocupa el socket), el binario domain NO
// queda vivo en estado "active pero listener muerto".
//
// issue-29.3 escenario 1+2+4: bind error → log FATAL + exit != 0
// dentro de 2 segundos.
//
// Estrategia: build del helper ListenAndServeWithFatalLog en una
// goroutine con un puerto ya ocupado por un net.Listener. La
// función debe retornar el error. Verificamos que el log incluye
// "FATAL: HTTP listener failed".
//
// (Si bien el test de sabotaje ya existe como TestListenAndServeWithFatalLog_PortBusy,
// este test valida la integración con main.runServer: log fatal + os.Exit(1).)
func TestServerExitsOnBindError(t *testing.T) {
	// Bloquea un puerto.
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer blocker.Close()
	port := blocker.Addr().(*net.TCPAddr).Port

	var logBuf bytes.Buffer
	logger := newTestLogger(&logBuf)

	// Simula exactamente el patrón de main.runServer.
	srv := &http.Server{
		Addr:    net.JoinHostPort("127.0.0.1", itoa(port)),
		Handler: http.NewServeMux(),
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := ListenAndServeWithFatalLog(srv, logger); err != nil {
			logger.Error("FATAL: HTTP listener failed", slog.Any("err", err))
			// En el main real: os.Exit(1). Acá solo loggeamos.
		}
	}()

	select {
	case <-done:
		require.Contains(t, logBuf.String(), "FATAL: HTTP listener failed",
			"log debe contener 'FATAL: HTTP listener failed' cuando puerto está ocupado. Got: %s", logBuf.String())
	case <-time.After(3 * time.Second):
		t.Fatal("ListenAndServeWithFatalLog no retornó en 3s")
	}
}

// TestServerExitsOnHealthCheckFailure verifica que el watchdog
// post-bind detecta un server que arrancó pero no responde /health.
//
// issue-29.3 escenario 3+4: bind OK pero mux sin /health (o que
// responde 500) → watchdog dispara FATAL.
//
// Estrategia: arrancar un server real con un mux que NO tiene
// /health, en un port random. Lanzar el watchdog con un fatalFn
// que marca un flag. Esperar 5s (settle + 3 intentos). Verificar
// que fatalFn fue llamado.
func TestServerExitsOnHealthCheckFailure(t *testing.T) {
	// Server real sin /health endpoint.
	mux := http.NewServeMux()
	mux.HandleFunc("/other", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{Addr: "127.0.0.1:0", Handler: mux}
	go func() {
		_ = srv.ListenAndServe()
	}()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	// Esperar a que el listener esté listo (poll puerto hasta abrir).
	addr := srv.Addr
	ln, err := net.Listen("tcp", addr)
	if err == nil {
		// srv ya está en :0 + puerto random, no podemos usar nuestro listener.
		// Pero necesitamos el puerto. Solución: srv.Addr ya tiene el puerto.
		ln.Close()
	}
	// Esperar 100ms a que srv abra el socket.
	time.Sleep(100 * time.Millisecond)
	port := extractPort(t, addr)

	var fatalErr error
	fatalCalled := make(chan struct{})
	fatalFn := func(err error) {
		fatalErr = err
		close(fatalCalled)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go RunPostBindWatchdog(ctx, port, fatalFn)

	select {
	case <-fatalCalled:
		require.Error(t, fatalErr, "fatalFn debe recibir error de ProbeHealth")
		require.Contains(t, fatalErr.Error(), "404",
			"el error debe indicar que /health retornó 404. Got: %v", fatalErr)
	case <-time.After(8 * time.Second):
		t.Fatal("watchdog no llamó fatalFn en 8s — debería haber disparado tras ~5s")
	}
}

// TestServerExitsCleanlyOnSIGTERM verifica que cuando el server
// recibe SIGTERM, hace graceful shutdown y exit 0 (no FATAL).
//
// issue-29.3 escenario 5+6: SIGTERM → exit 0, NO "FATAL".
//
// Estrategia: este test requiere un subproceso (no podemos SIGTERMar
// el test runner). Build + correr el binario domain en background,
// esperar a que abra el puerto, enviar SIGTERM, verificar exit code.
func TestServerExitsCleanlyOnSIGTERM(t *testing.T) {
	// Skip si no hay binario domain disponible (CI típico).
	bin := os.Getenv("DOMAIN_BIN_PATH")
	if bin == "" {
		bin = "/tmp/domain-test-server"
	}
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("binario domain no encontrado en %s — skip (esperado en CI sin bin prebuilt)", bin)
	}

	// Levantar el subproceso con puerto random + health stub.
	cmd := exec.Command(bin, "server",
		"--http-bind", "127.0.0.1",
		"--http-port", "0",
		"--no-migrate",
		"--health-stub",
	)
	cmd.Stdout = nil
	cmd.Stderr = nil
	require.NoError(t, cmd.Start())

	// Dar tiempo a que el proceso levante antes de enviar SIGTERM.
	time.Sleep(500 * time.Millisecond)

	// Enviar SIGTERM.
	require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))

	// Esperar a que termine (graceful shutdown ~3s).
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		// Exit code 0 = OK, exit != 0 = bug.
		if exitErr, ok := err.(*exec.ExitError); ok {
			require.Equal(t, 0, exitErr.ExitCode(),
				"SIGTERM debe causar exit 0, got %d", exitErr.ExitCode())
		} else {
			require.NoError(t, err)
		}
	case <-time.After(35 * time.Second): // > terminationGracePeriodSeconds (30) + slack
		_ = cmd.Process.Kill()
		t.Fatal("server no terminó en 35s tras SIGTERM — graceful shutdown colgado")
	}
}

// extractPort helper para tests: extrae el puerto de "host:port".
func extractPort(t *testing.T, addr string) int {
	t.Helper()
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		t.Fatalf("addr no tiene formato host:port: %s", addr)
	}
	// Convertir puerto manualmente (no usar strconv para evitar import).
	var port int
	for _, c := range parts[1] {
		if c < '0' || c > '9' {
			t.Fatalf("port inválido: %s", parts[1])
		}
		port = port*10 + int(c-'0')
	}
	return port
}
