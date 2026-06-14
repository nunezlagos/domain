# Design: issue-09.5-cloud-autosync

## Decisión arquitectónica

### State machine

```go
type SyncPhase string

const (
    PhaseIdle     SyncPhase = "idle"
    PhasePushing  SyncPhase = "pushing"
    PhasePulling  SyncPhase = "pulling"
    PhaseHealthy  SyncPhase = "healthy"
    PhaseFailed   SyncPhase = "failed"
    PhaseBackoff  SyncPhase = "backoff"
    PhaseDisabled SyncPhase = "disabled"
)

type ReasonCode string

const (
    ReasonNone         ReasonCode = ""
    ReasonNetworkError ReasonCode = "network_error"
    ReasonAuthError    ReasonCode = "auth_error"
    ReasonServerError  ReasonCode = "server_error"
    ReasonTimeout      ReasonCode = "timeout"
    ReasonRateLimited  ReasonCode = "rate_limited"
)

type SyncStatus struct {
    Phase       SyncPhase  `json:"phase"`
    ReasonCode  ReasonCode `json:"reason_code,omitempty"`
    LastPushAt  *time.Time `json:"last_push_at,omitempty"`
    LastPullAt  *time.Time `json:"last_pull_at,omitempty"`
    NextRetryAt *time.Time `json:"next_retry_at,omitempty"`
    PushCount   int64      `json:"push_count"`
    PullCount   int64      `json:"pull_count"`
    ErrorCount  int64      `json:"error_count"`
}
```

### Manager

```go
type AutosyncManager struct {
    mu       sync.RWMutex
    status   SyncStatus
    interval time.Duration
    backoff  []time.Duration
    backoffIdx int

    done     chan struct{}
    ticker   *time.Ticker
    enabled  bool

    pusher   SyncPusher
    puller   SyncPuller
}

func (m *AutosyncManager) Start(ctx context.Context) {
    if !m.enabled {
        m.setPhase(PhaseDisabled, ReasonNone)
        return
    }
    m.ticker = time.NewTicker(m.interval)
    go m.loop(ctx)
}

func (m *AutosyncManager) loop(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case <-m.done:
            return
        case <-m.ticker.C:
            m.runCycle(ctx)
        }
    }
}

func (m *AutosyncManager) runCycle(ctx context.Context) {
    m.setPhase(PhasePushing, ReasonNone)
    if err := m.pusher.Push(ctx); err != nil {
        m.handleError(err)
        return
    }
    m.status.LastPushAt = timeNow()

    m.setPhase(PhasePulling, ReasonNone)
    if err := m.puller.Pull(ctx); err != nil {
        m.handleError(err)
        return
    }
    m.status.LastPullAt = timeNow()

    m.setPhase(PhaseHealthy, ReasonNone)
    m.backoffIdx = 0
}

func (m *AutosyncManager) handleError(err error) {
    reason := classifyError(err)
    m.setPhase(PhaseFailed, reason)
    m.status.ErrorCount++
    m.backoffIdx = min(m.backoffIdx+1, len(m.backoff)-1)
    delay := m.backoff[m.backoffIdx]
    nextRetry := time.Now().Add(delay)
    m.status.NextRetryAt = &nextRetry

    time.AfterFunc(delay, func() {
        m.setPhase(PhaseBackoff, reason)
        m.runCycle(context.Background())
    })
}
```

### Backoff schedule

```go
var defaultBackoff = []time.Duration{
    30 * time.Second,
    60 * time.Second,
    120 * time.Second,  // 2min
    240 * time.Second,  // 4min
    300 * time.Second,  // 5min (max)
}
```

### Error classification

```go
func classifyError(err error) ReasonCode {
    if os.IsTimeout(err) {
        return ReasonTimeout
    }
    if strings.Contains(err.Error(), "connection refused") ||
       strings.Contains(err.Error(), "no such host") {
        return ReasonNetworkError
    }
    if strings.Contains(err.Error(), "401") ||
       strings.Contains(err.Error(), "403") {
        return ReasonAuthError
    }
    if strings.Contains(err.Error(), "429") {
        return ReasonRateLimited
    }
    if strings.Contains(err.Error(), "500") ||
       strings.Contains(err.Error(), "502") ||
       strings.Contains(err.Error(), "503") {
        return ReasonServerError
    }
    return ReasonNetworkError
}
```

### Status API

```go
func (m *AutosyncManager) StatusHandler(w http.ResponseWriter, r *http.Request) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    json.NewEncoder(w).Encode(m.status)
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Cron-based sync | Ticker (fixed interval) es más determinista y fácil de testear |
| Channel-based state machine | Enum + mutex es más simple y visible; channels agregan indirección |
| Sin backoff | Necesario para no saturar el server cuando hay errores transitorios |

## TDD plan

1. **Red:** Manager transiciona idle → pushing → pulling → healthy → falla
2. **Green:** Implement loop básico → pasa
3. **Red:** Push error → failed → backoff → pushing → falla
4. **Green:** Implement handleError + time.AfterFunc → pasa
5. **Red:** Disabled detiene manager → falla
6. **Green:** Implement PhaseDisabled check → pasa
7. **Sabotaje:** No incrementar backoffIdx → retry siempre a 30s → test de backoff falla → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Goroutine leak en Stop | done channel + select en loop; context cancel adicional |
| Race en status | sync.RWMutex en todos los accesos a status |
| Backoff infinito si siempre falla | backoffIdx no supera max (5min); error_count tracking para alerta |
| Push/Pull concurrente | Manager es single-goroutine; no hay concurrencia interna |
