# Design: HU-26.7-cache-invalidation-patterns

## API

```go
package distributed

type Cache interface {
  Get(key string) (any, bool)
  Set(key string, val any, ttl time.Duration)
  Delete(key string)
  Flush()
}

func WithInvalidation(c Cache, listener *Listener, channel string) Cache
```

## SQL function

```sql
CREATE OR REPLACE FUNCTION notify_cache_invalidation()
RETURNS TRIGGER AS $$
DECLARE
  payload TEXT;
BEGIN
  payload := jsonb_build_object(
    'op', TG_OP,
    'id', COALESCE(NEW.id, OLD.id)::text,
    'org_id', COALESCE(NEW.organization_id, OLD.organization_id)::text
  )::text;
  PERFORM pg_notify('cache_invalidate_' || TG_TABLE_NAME, payload);
  RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Helper para crear trigger en tabla específica
CREATE OR REPLACE FUNCTION create_cache_invalidation_trigger(table_name TEXT)
RETURNS void AS $$
BEGIN
  EXECUTE format($EXEC$
    DROP TRIGGER IF EXISTS notify_cache_inv ON %I;
    CREATE TRIGGER notify_cache_inv
      AFTER INSERT OR UPDATE OR DELETE ON %I
      FOR EACH ROW EXECUTE FUNCTION notify_cache_invalidation();
  $EXEC$, table_name, table_name);
END;
$$ LANGUAGE plpgsql;

-- usage in migrations:
SELECT create_cache_invalidation_trigger('custom_roles');
SELECT create_cache_invalidation_trigger('platform_policies');
SELECT create_cache_invalidation_trigger('mcp_tool_configs');
SELECT create_cache_invalidation_trigger('plans');
SELECT create_cache_invalidation_trigger('model_registry');
SELECT create_cache_invalidation_trigger('agents');
```

## Listener

```go
type Listener struct {
  conn      *pgx.Conn  // session conn, NOT pgbouncer txn
  handlers  map[string][]func(payload string)
  reconnects atomic.Int64
}

func (l *Listener) Listen(ctx context.Context, channel string, h func(string)) error {
  l.conn.Exec(ctx, "LISTEN " + channel)
  l.handlers[channel] = append(l.handlers[channel], h)
  return nil
}

func (l *Listener) Run(ctx) {
  for {
    n, err := l.conn.WaitForNotification(ctx)
    if err != nil { l.reconnect(); continue }
    for _, h := range l.handlers[n.Channel] { h(n.Payload) }
  }
}

func (l *Listener) reconnect() {
  l.reconnects.Inc()
  // close + reopen + re-LISTEN all channels + flush all caches
}
```

## TDD plan

1. Update tabla → NOTIFY publish
2. Listener recibe + invalidate
3. Multi-pod: 2 listeners reciben mismo NOTIFY
4. Reconnect → flush all + re-LISTEN
5. Dedupe 100ms
6. Métricas
