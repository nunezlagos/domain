// HTTP listener wrapper para issue-29.3 (boot-fail-loud).
//
// El bug detectado 2026-06-12: srv.ListenAndServe() en goroutine
// principal puede fallar silenciosamente si el puerto está ocupado
// o si hay un error de bind. El proceso sigue vivo (logs de
// otras goroutines como "runtime config refreshed" aparecen) pero
// el listener HTTP no responde. systemd reporta "active" porque
// el PID existe.
//
// La solución: escuchar el resultado de ListenAndServe en un
// canal con timeout. Si hay error != nil, log FATAL + return.
// Si el listener arrancó pero no responde health, el watchdog
// post-bind lo detecta.
package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// ListenAndServeWithFatalLog corre srv.ListenAndServe() en una
// goroutine. Si retorna error != nil (y != http.ErrServerClosed),
// loggea con nivel ERROR + mensaje "FATAL: HTTP listener failed"
// Y retorna el error. El caller decide si hace os.Exit(1).
//
// Si el listener arranca OK (no hay error), retorna nil después
// de un breve settle. El watchdog post-bind (en otra goroutine)
// es responsable de detectar "arrancó pero no responde health".
//
// Esta función es bloqueante: NO retorna mientras el server
// está sirviendo. Solo retorna en error o shutdown limpio.
func ListenAndServeWithFatalLog(srv *http.Server, logger *slog.Logger) error {
	listenErrCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			listenErrCh <- err
		} else {
			listenErrCh <- nil
		}
	}()

	err := <-listenErrCh
	if err != nil {
		if logger != nil {
			logger.Error("FATAL: HTTP listener failed", slog.Any("err", err))
		}
		return err
	}
	return nil
}

// RunPostBindWatchdog corre el watchdog que verifica que el server
// responda a /health después de un breve settle. Si los N intentos
// fallan, llama a fatalFn (típicamente una func que loggea y hace
// os.Exit(1)). Es fire-and-forget: el caller hace `go RunPostBindWatchdog(...)`.
//
// Defaults: 2s settle + 3 intentos con 1s entre cada uno.
func RunPostBindWatchdog(ctx context.Context, port int, fatalFn func(error)) {
	if fatalFn == nil {
		return
	}
	select {
	case <-time.After(2 * time.Second):
	case <-ctx.Done():
		return
	}
	var lastErr error
	for i := 0; i < 3; i++ {
		if err := ProbeHealth(port); err == nil {
			return
		} else {
			lastErr = err
		}
		select {
		case <-time.After(1 * time.Second):
		case <-ctx.Done():
			return
		}
	}
	fatalFn(lastErr)
}
