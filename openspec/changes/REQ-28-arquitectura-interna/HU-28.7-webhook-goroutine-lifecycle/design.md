# Design: HU-28.7-webhook-goroutine-lifecycle

## Worker pool

```go
// internal/api/handler/webhook.go

type webhookJob struct {
    ctx       context.Context
    webhookID uuid.UUID
    payload   json.RawMessage
    timestamp time.Time
}

const (
    webhookWorkerPoolSize = 10
    webhookQueueSize      = 100
    webhookDispatchTimeout = 30 * time.Second
)

type WebhookHandler struct {
    dispatchCh chan webhookJob
    wg         sync.WaitGroup
    cancel     context.CancelFunc
}

func NewWebhookHandler() *WebhookHandler {
    ctx, cancel := context.WithCancel(context.Background())
    h := &WebhookHandler{
        dispatchCh: make(chan webhookJob, webhookQueueSize),
        cancel:     cancel,
    }
    for i := 0; i < webhookWorkerPoolSize; i++ {
        h.wg.Add(1)
        go h.worker(ctx)
    }
    return h
}

func (h *WebhookHandler) worker(ctx context.Context) {
    defer h.wg.Done()
    for {
        select {
        case <-ctx.Done():
            return
        case job := <-h.dispatchCh:
            dispatchCtx, cancel := context.WithTimeout(job.ctx, webhookDispatchTimeout)
            h.dispatchWebhook(dispatchCtx, job.webhookID, job.payload)
            cancel()
        }
    }
}

func (h *WebhookHandler) Shutdown(ctx context.Context) error {
    h.cancel()
    done := make(chan struct{})
    go func() {
        h.wg.Wait()
        close(done)
    }()
    select {
    case <-done:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

## Integración con graceful shutdown

```go
// cmd/domain/main.go
whHandler := handler.NewWebhookHandler()
// ... en shutdown:
if err := whHandler.Shutdown(shutdownCtx); err != nil {
    slog.Error("webhook handler shutdown", "error", err)
}
```
