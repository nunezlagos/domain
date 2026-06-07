# Design: HU-26.4-graceful-shutdown

## Coordinator

```go
package shutdown

type Coordinator struct {
  httpServer   *http.Server
  workerCancel context.CancelFunc
  pools        []*pgxpool.Pool
  readyState   *atomic.Bool
}

func (c *Coordinator) Run(ctx context.Context) error {
  // 1. Flip readiness OFF
  c.readyState.Store(false)
  slog.Info("readiness disabled, draining")
  
  // 2. Grace ELB drain
  time.Sleep(5 * time.Second)
  
  // 3. HTTP shutdown (con timeout)
  httpCtx, _ := context.WithTimeout(ctx, 25*time.Second)
  if err := c.httpServer.Shutdown(httpCtx); err != nil {
    slog.Error("http shutdown forced", "err", err)
  }
  
  // 4. Workers cancel
  c.workerCancel()
  // wait workers con timeout
  awaitWorkers(20 * time.Second)
  
  // 5. Pool close
  for _, p := range c.pools { p.Close() }
  
  metrics.ShutdownComplete(time.Since(start))
  return nil
}
```

## Signal handling

```go
// cmd/domain-mcp/main.go
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
<-sigCh
coordinator.Run(context.Background())
os.Exit(0)
```

## Readiness probe

```go
// internal/http/handlers/health.go
func (h *Health) ReadyHandler(w http.ResponseWriter, r *http.Request) {
  if !h.readyState.Load() {
    w.WriteHeader(http.StatusServiceUnavailable)
    return
  }
  // also check DB ping with 1s timeout
  if err := h.db.Ping(...); err != nil {
    w.WriteHeader(http.StatusServiceUnavailable)
    return
  }
  w.WriteHeader(http.StatusOK)
}
```

## TDD plan

1. Signal simulado → orden correcto
2. HTTP in-flight 5s → termina antes que pool close
3. Worker mid-iter → context cancel + checkpoint
4. Timeout total → forced log
5. /health/ready 503 durante drain
6. /health liveness 200 hasta exit
7. Métricas observables
