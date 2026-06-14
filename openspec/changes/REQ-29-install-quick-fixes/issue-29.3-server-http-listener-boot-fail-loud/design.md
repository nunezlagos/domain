# Design: issue-29.3-server-http-listener-boot-fail-loud

## Contexto

Bug detectado en sesión 2026-06-12:
- `systemctl --user status domain.service` → "active (running)"
- `ss -ltn | grep :8000` → VACÍO
- `curl http://localhost:8000/health` → connection refused (exit 7)
- `journalctl -u domain.service` → muestra el proceso loggeando "runtime config refreshed" en loop

El proceso está VIVO (systemd lo ve por su PID), pero el listener HTTP
nunca abrió :8000. Cualquier agente que intente usar el MCP server recibe
"MCP error -32000: Connection closed".

**Análisis del código actual** (`cmd/domain/main.go:915`):

```go
if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
    logger.Error("server failed", slog.Any("err", err))
    os.Exit(1)
}
```

Este código PARECE correcto. Pero la causa raíz probable es que
`srv.ListenAndServe()` está en el main goroutine y bloquea. Si la
goroutine del server PANICS (e.g. por un nil pointer en un handler
registrado tarde), el `ListenAndServe` retorna. El `os.Exit(1)` se
ejecuta. PERO si hay un `defer recover()` en algún middleware o
wrapper, el panic se atrapa y la ejecución continúa — el flujo
imprime "runtime config refreshed" en loop (de las goroutines
background) y nunca muere.

Otra causa plausible: el `defer pools.Close()` o el
`defer schedCancel()` se ejecutan en orden de registro inverso. Si el
servidor falló antes de registrar el `defer`, el binario hace exit
pero systemd tarda en reflejarlo. Con `Restart=always` (default de
algunos setups), systemd lo relanza, el puerto sigue ocupado, vuelve a
fallar, loop de restart sin listener útil.

## Decisión arquitectónica

**Estrategia:** boot-loud + health-check post-bind.

1. **Detección explícita de bind failure:** envolver
   `srv.ListenAndServe()` en un `errgroup` o un canal que reciba el
   error. Si retorna `!= http.ErrServerClosed`, loggear con
   `slog.Error` Y `os.Exit(1)` **sin demoras**.

2. **Health-check post-bind (watchdog):** inmediatamente después de que
   `srv.ListenAndServe()` retorne exitosamente (o en paralelo, en una
   goroutine separada con `time.Sleep(2*time.Second)`), hacer un
   `GET http://127.0.0.1:<port>/health` con timeout 1s. Si falla
   3 veces consecutivas, `logger.Error("health-check post-bind failed
   3x")` + `os.Exit(1)`.

3. **Panic recovery en handlers:** agregar `defer recover()` en el
   middleware raíz de `mux`, para que un panic en un handler no tumbe
   el server silenciosamente. Loggear con `slog.Error` + `panic value`
   + stack trace.

4. **Test de integración con port ocupado:** el test e2e bootea un
   fake listener en :8000, luego intenta `domain server` en otra
   instancia (o en una goroutine del test) y verifica que sale con
   exit != 0 y mensaje "FATAL" en <3s.

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Confiar solo en `os.Exit(1)` existente (status quo) | El bug YA está pasando con este código aparentemente correcto. No es suficiente. |
| B | Watchdog externo (un script `domain-watcher.sh` que reinicia si health falla) | Agrega un proceso más al systemd unit, complica el deploy, race conditions entre el script y el binario. |
| C | `Type=notify` en systemd + `sd_notify(READY=1)` post-bind | Es la forma "correcta" en systemd, pero requiere libsystemd o un wrapper. Mucho más invasivo que un health-check interno. |
| D | Container readiness probe (solo aplica en K8s) | No aplica a dev local con systemd, que es el caso del bug. |

## Por qué la combinación "boot-loud + health-check post-bind" gana

- **Defensa en profundidad:** si el bind error se silencia en una
  capa, el health-check lo agarra en la otra.
- **Self-contained:** no requiere systemd `Type=notify` ni
  sidecars. Funciona en cualquier entorno donde corra el binario.
- **Observable:** el log "FATAL: HTTP listener failed" o "FATAL:
  health-check failed 3x" es inequívoco para el operador. No más
  "active but no listener" silencioso.
- **Testeable:** el test e2e ocupa el puerto, bootea el server, y
  asserta exit 1 + mensaje. Sin esto no podíamos garantizar que el
  fix resuelve el bug.

## Detalle de implementación

```go
// En runServer, reemplazar el bloque actual de ListenAndServe:

listenErrCh := make(chan error, 1)
go func() {
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        listenErrCh <- err
    } else {
        listenErrCh <- nil
    }
}()

// Watchdog: health-check post-bind
go func() {
    time.Sleep(2 * time.Second)
    for i := 0; i < 3; i++ {
        if err := probeHealth(cfg.HTTPPort); err == nil {
            return // OK
        }
        time.Sleep(1 * time.Second)
    }
    logger.Error("FATAL: health-check post-bind failed 3x — listener not responding")
    os.Exit(1)
}()

if err := <-listenErrCh; err != nil {
    logger.Error("FATAL: HTTP listener failed", slog.Any("err", err))
    os.Exit(1)
}
```

## Riesgos

- **R1:** El watchdog puede dispararse falsamente si el server tarda
  más de 5s en arrancar bajo carga. **Mitigación:** 3 reintentos con
  1s entre cada uno = 5s totales. El startup actual de `domain server`
  es <1s en dev. En prod con mucho seed puede ser más; ajustar a
  `cfg.HTTPReadinessTimeout` si es necesario.
- **R2:** El `go func()` adicional agrega complejidad. **Aceptable:**
  es el costo de tener observabilidad real. La alternativa (dejar el
  bug) es peor.

## Sabotaje test (referencia)

Romper la lógica de FATAL (e.g. comentar `os.Exit(1)` en el `if err
!= nil`) → test que asserta exit != 0 cuando se ocupa el puerto DEBE
FALLAR → restaurar → test verde. Documentar en commit body.
