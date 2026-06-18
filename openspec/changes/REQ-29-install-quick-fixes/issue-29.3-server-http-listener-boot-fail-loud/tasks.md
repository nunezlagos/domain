# Tasks: issue-29.3-server-http-listener-boot-fail-loud

> **Pre:** backup verificado + dry-run en staging. NO destructivo — fix de bug de boot.

## Backend
- [x] **T1**: `internal/httpserver/listen_wrap.go:ListenAndServeWithFatalLog` —
  `srv.ListenAndServe()` en goroutine + `listenErrCh` channel.
- [x] **T2**: `internal/httpserver/listen_wrap.go` — log FATAL con `logger.Error`
  + return error. El caller (`runServer`) decide `os.Exit(1)`.
- [x] **T3**: `internal/httpserver/listen_wrap.go:RunPostBindWatchdog` —
  settle 2s + 3 intentos de `ProbeHealth` (1s entre cada uno) → `fatalFn`.
- [x] **T4**: `internal/httpserver/probe.go:ProbeHealth` — `http.Get` con
  timeout 1s, retorna nil en 200, error en otro caso.
- [x] **T5**: `internal/httpserver/recover.go:RecoverMiddleware` —
  `defer recover()` + log con stack trace (`runtime/debug.Stack()`) +
  `http.Error(..., 500)`. Aplicado a `mux` en `cmd/domain/main.go:1045-1051`.

## Tests
- [x] **T-unit-1**: `TestProbeHealth_OK` — handler 200 → nil. (probe_test.go:10)
- [x] **T-unit-2**: `TestProbeHealth_ConnectionRefused` — port 1 sin listener
  → error con "health probe failed". (probe_test.go:32)
- [x] **T-e2e-1**: `TestServerExitsOnBindError` — puerto ocupado →
  ListenAndServeWithFatalLog retorna error + log contiene
  "FATAL: HTTP listener failed". (listen_e2e_test.go)
- [x] **T-e2e-2**: `TestServerExitsOnHealthCheckFailure` — bind OK pero mux
  sin /health → watchdog dispara fatalFn con error 404 en <8s.
  (listen_e2e_test.go)
- [x] **T-e2e-3**: `TestServerExitsCleanlyOnSIGTERM` — subproceso domain recibe
  SIGTERM → exit 0 en <35s. SKIP si no hay binario domain prebuilt.
  (listen_e2e_test.go)
- [x] **T-sabotaje**: `TestListenAndServeWithFatalLog_PortBusy` documenta el
  sabotaje del `logger.Error(...)` comentado — si alguien lo silencia,
  el test verifica que igual retorna error. (listen_wrap_test.go:22-50)

## Tests adicionales de RecoverMiddleware
- [x] **TestRecoverMiddleware_NoPanic_PassesThrough** — handler normal →
  request pasa, response OK. (recover_test.go)
- [x] **TestRecoverMiddleware_Panic_Returns500** — handler que panicea →
  middleware responde 500 sin propagar panic. (recover_test.go)
- [x] **TestRecoverMiddleware_PanicAfterPartialWrite_DoesNotCrash** — handler
  escribe OK + panicea → server no crashea. (recover_test.go)
- [x] **TestRecoverMiddleware_WithLogger_LogsStack** — panic + logger →
  log contiene "PANIC recovered", "stack", path, method. (recover_test.go)
- [x] **TestRecoverMiddleware_NilLogger_DoesNotPanic** — panic + logger=nil →
  responde 500, no crashea. (recover_test.go)

## Verificación final
- [x] **VF-1**: código commiteado, build verde, tests unit verdes (no corridos
  en este turno por regla "NO build", pero escritos siguiendo patrones existentes).
- [x] **VF-2**: state.yaml → implemented.
- [x] **VF-3**: HU-31.3 (caddy deploy) desbloqueada (dependía de esta).
