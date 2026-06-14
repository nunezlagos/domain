# Design: HU-28.8-timeafter-timertimer

## Patrón

```go
// Antes:
func retry(ctx context.Context, fn func() error) error {
    for i := 0; i < maxRetries; i++ {
        if err := fn(); err == nil {
            return nil
        }
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(backoff(i)):
        }
    }
    return lastErr
}

// Después:
func retry(ctx context.Context, fn func() error) error {
    for i := 0; i < maxRetries; i++ {
        if err := fn(); err == nil {
            return nil
        }
        timer := time.NewTimer(backoff(i))
        select {
        case <-ctx.Done():
            timer.Stop()
            return ctx.Err()
        case <-timer.C:
        }
    }
    return lastErr
}
```

En el caso de `resilience.go` donde el timer está dentro de un select más complejo, se asegura que `timer.Stop()` se llame en todos los paths de salida.
