# issue-29.3-server-http-listener-boot-fail-loud

**Origen:** `REQ-29-install-quick-fixes`
**Prioridad tentativa:** crítica
**Tipo:** fix (bug serio)

## Historia de usuario

**Como** operador de un `domain.service` en systemd
**Quiero** que el binario FALLE ruidosamente y ponga el unit en `failed` si el listener HTTP no logra abrir el puerto
**Para** no tener un servicio "active" corriendo sin responder en :8000 (caso detectado 2026-06-12: `ss -ltn` vacío, `curl /health` → 000, `systemctl status domain.service` → "active (running)")

## Criterios de aceptación

### Escenario 1: Puerto ocupado → binario exit 1 con error explícito

```gherkin
Dado que ya hay un proceso escuchando en :8000 (e.g. otra instancia de `domain server`)
Cuando corro `domain server` (intenta bind a :8000)
Entonces el proceso termina con exit 1 en <2 segundos
Y se imprime por stderr: "FATAL: HTTP listener failed: <error de bind>" con `log.Error` (slog)
Y NO se imprimen logs subsecuentes de "runtime config refreshed" (porque el proceso ya murió)
```

### Escenario 2: Systemd refleja el estado correcto

```gherkin
Dado que el puerto :8000 está ocupado
Y `domain.service` está configurado con `Type=simple` y `Restart=no` (o `Restart=on-failure`)
Cuando systemd intenta arrancar el servicio
Entonces `systemctl status domain.service` muestra "failed" o "inactive (dead)" (NO "active (running)")
Y `journalctl -u domain.service` muestra la línea "FATAL: HTTP listener failed"
```

### Escenario 3: Detección de bind-error silencioso (hipótesis A)

```gherkin
Dado que `srv.ListenAndServe()` retorna `err != nil && err != http.ErrServerClosed`
Cuando el flujo en `runServer` (cmd/domain/main.go:915) entra al if de error
Entonces se llama `logger.Error("server failed", ...)` y luego `os.Exit(1)` SIN RETARDO
Y el stack trace queda en el log (slog con AddSource=true)
```

### Escenario 4: Sabotaje — bind error silenciado

```gherkin
Dado que `srv.ListenAndServe()` retorna error (e.g. puerto ocupado, forzado con un mock o un port ya en uso)
Y el código de error fue REESCRITO a `_ = err` (sabotaje)
Cuando corro `domain server`
Entonces el proceso SIGUE CORRIENDO (bug actual: sigue vivo, no muere)
Y el test `TestServerExitsOnBindError` DEBE FALLAR (afirma exit != 0)
Cuando restauro el `os.Exit(1)` y el log explícito
Entonces el proceso muere en <2s con el mensaje FATAL
```

### Escenario 5: Edge case — error legítimo de shutdown

```gherkin
Dado que el server está corriendo y recibe SIGTERM
Y `srv.Shutdown(ctx)` retorna `http.ErrServerClosed` (caso normal de shutdown)
Cuando esto ocurre
Entonces el proceso termina con exit 0 (no 1)
Y NO se imprime "FATAL: HTTP listener failed"
```

### Escenario 6: Edge case — server arranca OK y shutdown por SIGTERM

```gherkin
Dado que el puerto :8000 está libre
Cuando corro `domain server` y después de 1s le mando SIGTERM
Entonces el proceso termina con exit 0
Y el log muestra "shutdown signal received" + "graceful shutdown complete"
Y `srv.ListenAndServe()` retorna `http.ErrServerClosed` (NO entra al FATAL)
```

## Notas

- El handler actual `if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed { ... os.Exit(1) }` en `cmd/domain/main.go:915` PARECE correcto, pero el bug real es que en algunas condiciones (e.g. port ocupado por otro binario que ocupa + libera, race con el systemd auto-restart, o un `_ = err` olvidado en otra rama), el proceso queda zombie "active" sin listener.
- El fix es **doble**: (a) asegurar que CUALQUIER path de error en el boot hace exit no-cero, (b) agregar un health-check post-boot de 2-3 segundos que verifica que el puerto está abierto; si no, log.Fatal explícito.
- Ver también: el install_progress en `runInstall` loggea "domain.service enabled + running" — pero ese log es del `systemctl --user start`, no del boot del binario. La aserción "running" viene de systemd reportando PID vivo, no del health-check.
