# HU-28.7-webhook-goroutine-lifecycle

**Origen:** `REQ-28-arquitectura-interna`
**Prioridad tentativa:** media
**Tipo:** fix

## Historia de usuario

**Como** operador de Domain
**Quiero** que el dispatch de webhooks use `context.Context` con timeout y `sync.WaitGroup` para controlar el lifecycle de las goroutines
**Para** que un flood de webhooks no cause unbounded goroutine growth, y las goroutines no queden huerfanas si el servidor se apaga

## Contexto

En `internal/api/handler/webhook.go:61` y `webhook_admin.go:212`:
```go
go a.dispatchWebhook(context.Background(), ...)
```
Esto tiene 3 problemas:
1. **Unbounded goroutines**: cada webhook entrante lanza una goroutine. Si llegan 10k webhooks por segundo, hay 10k goroutines compitiendo.
2. **Sin cancelación**: usa `context.Background()` — no hereda el context del request HTTP, no se puede cancelar desde afuera.
3. **Sin WaitGroup**: el graceful shutdown no puede esperar a que terminen los webhooks en vuelo.

## Criterios de aceptación

### Escenario 1: Timeout por webhook

```gherkin
Dado que un dispatchWebhook tarda más de 30s
Cuando el timeout se dispara
Entonces la goroutine se cancela via context
Y no queda huerfana
```

### Escenario 2: Graceful shutdown espera webhooks

```gherkin
Dado que hay 3 webhooks en vuelo
Cuando el server recibe SIGTERM
Entonces el graceful shutdown espera hasta que los 3 terminen (o hasta el timeout global)
```

### Escenario 3: Backpressure — cola acotada

```gherkin
Dado que llegan más webhooks de los que la cola puede manejar
Cuando la cola está llena
Entonces el handler responde 503 Service Unavailable inmediatamente
Y no se lanza una goroutine nueva
```

## Análisis breve

- **Qué pide:** Worker pool pattern o bounded channel para webhook dispatch. Timeout por webhook. WaitGroup para shutdown.
- **Módulos afectados:** `internal/api/handler/webhook.go`, `webhook_admin.go`, `cmd/domain/main.go` (shutdown)
- **Esfuerzo tentativo:** M (1-2 días)
- **Dependencias:** Ninguna
