# Tasks: issue-29.3-server-http-listener-boot-fail-loud

## Backend

- [ ] **T1**: En `cmd/domain/main.go:915` (runServer), extraer
  `srv.ListenAndServe()` a una goroutine que envíe el resultado a un
  canal `listenErrCh := make(chan error, 1)`. El main goroutine
  espera con `<-listenErrCh`.

- [ ] **T2**: Reemplazar el bloque actual de error handling por
  logging explícito: `logger.Error("FATAL: HTTP listener failed",
  slog.Any("err", err))` seguido de `os.Exit(1)`. Sin demoras, sin
  `defer`s intermedios. Usar `slog` con `AddSource` para capturar
  archivo:línea.

- [ ] **T3**: Agregar watchdog post-bind en otra goroutine:
  - `time.Sleep(2 * time.Second)` (deja al server armar el mux + DB
    pool).
  - Loop de 3 intentos de `GET http://127.0.0.1:<cfg.HTTPPort>/health`
    con timeout 1s.
  - Si los 3 fallan: `logger.Error("FATAL: health-check post-bind
    failed 3x")` + `os.Exit(1)`.

- [ ] **T4**: Helper `probeHealth(port int) error` en
  `internal/httpserver/probe.go` (nuevo archivo). Usa `http.Client`
  con timeout 1s. Retorna `nil` en 200, error en otro caso (incluido
  connection refused).

- [ ] **T5**: Agregar `defer func() { if r := recover(); r != nil {
  logger.Error("PANIC recovered in HTTP handler", slog.Any("panic",
  r), slog.String("stack", string(debug.Stack()))) } }()` al top del
  handler raíz. Evita que un panic en un handler tumbe el server
  silenciosamente.

## Tests

- [ ] **T-unit-1**: `TestProbeHealth_OK` — server real en puerto random
  + handler `/health` que responde 200 → `probeHealth` retorna nil.
- [ ] **T-unit-2**: `TestProbeHealth_ConnectionRefused**` — puerto
  random sin listener → `probeHealth` retorna error con "connection
  refused" en el mensaje.
- [ ] **T-e2e-1**: `TestServerExitsOnBindError**` — bootea un fake
  `net.Listen("tcp", ":8000")` (o puerto random) que mantiene el puerto
  ocupado → corre `runServer` en una goroutine → asserta que en <3s el
  proceso termina con exit != 0 y que el log capturado contiene
  "FATAL: HTTP listener failed".
- [ ] **T-e2e-2**: `TestServerExitsOnHealthCheckFailure**` — boot
  exitoso del listener pero con un mux que NO tiene `/health` (o que
  lo responde 500) → watchdog debe disparar FATAL y exit != 0.
  Variante: usar un `mux` mockeado.
- [ ] **T-e2e-3**: `TestServerExitsCleanlyOnSIGTERM**` — `domain server`
  corriendo, después de 1s enviar SIGTERM (vía `syscall.Kill`) → exit
  0, NO "FATAL" en log, mensaje "graceful shutdown complete".
- [ ] **T-sabotaje**: Comentar la línea `os.Exit(1)` en T2 (simulando
  el `_ = err` original) → correr T-e2e-1 → DEBE ver que el test
  falla (exit sigue siendo 0 o timeout) → restaurar `os.Exit(1)` →
  test verde. Documentar el sabotaje en commit body: "rompimos el
  guard explícitamente para confirmar que el test lo caza".
