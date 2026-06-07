# Design: HU-12.6-mcp-tool-resilience

## Schema

```sql
CREATE TABLE mcp_tool_configs (
  slug VARCHAR(100) PRIMARY KEY,
  description TEXT,
  is_write BOOLEAN NOT NULL DEFAULT false,
  cacheable BOOLEAN NOT NULL DEFAULT true,
  cache_ttl_seconds INT NOT NULL DEFAULT 300,
  timeout_ms INT NOT NULL DEFAULT 5000,
  retry_count INT NOT NULL DEFAULT 1,        -- 0 para writes
  retry_backoff_ms INT NOT NULL DEFAULT 100,
  circuit_threshold INT NOT NULL DEFAULT 5,
  circuit_reset_seconds INT NOT NULL DEFAULT 60,
  enabled BOOLEAN NOT NULL DEFAULT true,
  updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

## Middleware chain

```go
// internal/mcp/resilience/middleware.go
type Middleware func(next ToolHandler) ToolHandler

func WithResilience(reg *Registry) Middleware {
  return func(next ToolHandler) ToolHandler {
    return func(ctx context.Context, args ToolArgs) (ToolResult, error) {
      cfg := reg.Get(args.ToolName)
      if !cfg.Enabled { return nil, ErrToolDisabled }
      
      // 1. Cache check (only for cacheable + idempotent)
      if cfg.Cacheable {
        if r, ok := reg.cache.Lookup(args); ok && !r.Stale() {
          return r.Fresh()
        }
      }
      
      // 2. Circuit breaker
      cb := reg.breakers.Get(args.ToolName)
      if cb.State() == Open {
        // serve degraded if cache available
        if cfg.Cacheable {
          if r, ok := reg.cache.Lookup(args); ok {
            return r.Degraded()
          }
        }
        return nil, ErrCircuitOpen
      }
      
      // 3. Timeout
      ctx, cancel := context.WithTimeout(ctx, time.Duration(cfg.TimeoutMs)*time.Millisecond)
      defer cancel()
      
      // 4. Retry transitorio (only non-writes)
      var result ToolResult
      var err error
      attempts := 1
      if !cfg.IsWrite { attempts = 1 + cfg.RetryCount }
      for i := 0; i < attempts; i++ {
        result, err = next(ctx, args)
        if err == nil || !isTransient(err) { break }
        if i+1 < attempts {
          time.Sleep(time.Duration(cfg.RetryBackoffMs) * time.Millisecond)
        }
      }
      
      // 5. Track CB + cache
      if err != nil {
        cb.RecordFailure()
      } else {
        cb.RecordSuccess()
        if cfg.Cacheable { reg.cache.Set(args, result, cfg.CacheTTL()) }
      }
      
      return result, err
    }
  }
}
```

## Cache local

`hashicorp/golang-lru/v2` o equivalente con max 10000 entries por tool, TTL configurable.

Key: `sha256(tool_name + json.Marshal(args_canonical))`.

Stale entry sirve `degraded` cuando CB OPEN o DB hipo.

## Métricas

```
domain_mcp_tool_calls_total{tool,status,source}    counter
domain_mcp_tool_duration_seconds{tool}             histogram
domain_mcp_tool_timeouts_total{tool}               counter
domain_mcp_tool_retries_total{tool}                counter
domain_mcp_circuit_state{tool}                     gauge 0/1/2
domain_mcp_cache_hits_total{tool,mode}             counter (mode: fresh|degraded)
domain_mcp_degraded_responses_total{tool}          counter
```

## Config reload

```go
// LISTEN/NOTIFY 'mcp_tool_config_changed'
listener.Listen(ctx, "mcp_tool_config_changed", func(n Notify) {
  reg.ReloadConfig(ctx, n.Slug)
})

// trigger SQL
ALTER TABLE mcp_tool_configs ENABLE TRIGGER ...
CREATE TRIGGER notify_config_change AFTER UPDATE OR INSERT ON mcp_tool_configs
  FOR EACH ROW EXECUTE FUNCTION pg_notify('mcp_tool_config_changed', NEW.slug::text);
```

## TDD plan

1. Timeout 2s + slow query 3s → tool_timeout error
2. 5 errores consecutivos → CB open
3. CB open + cache hit → degraded response
4. CB open + no cache → service_unavailable
5. Cache fresh hit → no DB call
6. Retry transitorio 5xx → success en 2do attempt
7. Write tool sin retry
8. Reload config NOTIFY observable
9. DB sabotaje suite: drop conn, slow query, kill query → cada tool maneja apropiado
10. Métricas todas se publican
