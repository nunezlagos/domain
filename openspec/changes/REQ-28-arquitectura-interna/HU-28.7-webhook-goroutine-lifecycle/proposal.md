# Proposal: HU-28.7-webhook-goroutine-lifecycle

## Intención

Reemplazar `go dispatch(context.Background(), ...)` por un worker pool acotado con context derivado y WaitGroup. Sin perder la semántica de "disparar y olvidar".

## Scope

**Incluye:**
- Worker pool con buffer acotado (ej: 100) para dispatch de webhooks
- Context con timeout (30s) por dispatch
- WaitGroup integrado al graceful shutdown
- 503 cuando la cola está llena

**No incluye:**
- Retry/backoff para webhooks fallidos (ya existe en dispatchWebhook)
- Persistencia de webhooks fallidos en DLQ

## Enfoque

```go
type WebhookHandler struct {
    dispatchCh chan webhookJob
    wg         sync.WaitGroup
}

func NewWebhookHandler(poolSize int) *WebhookHandler {
    h := &WebhookHandler{
        dispatchCh: make(chan webhookJob, 100),
    }
    for i := 0; i < poolSize; i++ {
        go h.worker()
    }
    return h
}

func (h *WebhookHandler) handleReceive(w, r) {
    select {
    case h.dispatchCh <- job{ctx, webhookID, payload}:
        w.WriteHeader(http.StatusAccepted)
    default:
        w.WriteHeader(http.StatusServiceUnavailable)
    }
}
```
