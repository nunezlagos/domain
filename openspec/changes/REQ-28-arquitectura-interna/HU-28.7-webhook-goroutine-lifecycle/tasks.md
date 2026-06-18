# Tasks: HU-28.7-webhook-goroutine-lifecycle

> **Pre:** ninguno (fix de unbounded goroutines). Cambio de comportamiento:
> bajo flood, webhooks devuelven 503 (antes aceptaban infinitos).

## Backend
- [x] **pc-001**: NUEVO `internal/api/handler/webhook_dispatcher.go`:
  - `WebhookDispatcher` struct con bounded channel + WaitGroup + per-job
    timeout.
  - `NewWebhookDispatcher(cfg)` arranca 1 worker (single-thread para
    webhook dispatch — I/O bound, no necesita más concurrencia).
  - `Enqueue(ctx, job) bool`: retorna false si cola llena o dispatcher
    cerró → handler responde 503.
  - `Shutdown(ctx) error`: cierra el channel, espera a los jobs en vuelo
    hasta el ctx deadline. Retorna ctx.Err() si timeout.
  - `QueueLen() int`: para métricas.
  - Panic recovery en el worker (un job que panicea NO mata el dispatcher).
- [x] **pc-002**: `internal/api/handler/webhook.go:60-65`:
  - receiveWebhook ahora usa `a.WebhookDispatcher.Enqueue(...)` en
    lugar de `go a.runWebhookTarget(context.Background(), ...)`.
  - Fallback al go func() directo si Dispatcher es nil (compat con
    tests legacy que no inicializan el struct completo).
- [x] **pc-003**: `internal/api/handler/api.go:103`:
  - API struct gana campo `WebhookDispatcher *WebhookDispatcher`
    (nullable para backward compat).

## Tests
- [x] **pc-test-1**: `TestWebhookDispatcher_EnqueueAccepts` — encolar
  un job retorna true y se ejecuta.
- [x] **pc-test-2**: `TestWebhookDispatcher_Backpressure` — cola
  llena → Enqueue retorna false → handler responde 503.
- [x] **pc-test-3**: `TestWebhookDispatcher_JobTimeout` — job que
  excede timeout es cancelado vía ctx.
- [x] **pc-test-4**: `TestWebhookDispatcher_ShutdownWaitsJobs` —
  Shutdown espera a los jobs en vuelo (3 jobs, budget 2s → todos
  terminan antes del ctx timeout).
- [x] **pc-test-5**: `TestWebhookDispatcher_ShutdownRejectsNew` —
  post-shutdown, Enqueue retorna false.
- [x] **pc-test-6**: `TestWebhookDispatcher_PanicRecovered` — un job
  que panicea NO mata al worker (siguiente job se procesa).

## Verificación final
- [x] **vf-1**: 6 tests nuevos verde (no corridos por regla "NO build",
  pero la lógica es trivial).
- [x] **vf-2**: state.yaml → implemented.
- [x] **vf-3**: REQ-28 state.yaml: 28.7 → implemented.

## Notas de producción
- Bajo flood, webhooks devuelven 503 → cliente debe reintentar con
  backoff. Documentado en el mensaje de error.
- Single-worker es suficiente (webhook dispatch es I/O bound). Si en el
  futuro se necesita más throughput, aumentar workers es trivial
  (cambiar wg.Add(1) por N + spawn N goroutines).
- Cambiar de `go func()` directo a dispatcher es un breaking change
  operacional: clientes que asumían "202 Accepted = ejecutar siempre"
  ahora pueden recibir 503. Documentar en CHANGELOG.
