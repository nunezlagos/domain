# Design: HU-27.3-hot-reload-config

## Schema

```sql
CREATE TABLE runtime_configs (
  key VARCHAR(100) PRIMARY KEY,
  value JSONB NOT NULL,
  default_value JSONB NOT NULL,
  is_hot_reloadable BOOLEAN NOT NULL DEFAULT false,
  description TEXT,
  last_changed_by UUID REFERENCES users(id),
  last_changed_at TIMESTAMPTZ DEFAULT NOW()
);

SELECT create_cache_invalidation_trigger('runtime_configs');  -- HU-26.7
```

## Apply hooks registry

```go
type ConfigHook interface {
  Key() string
  Validate(value any) error
  Apply(ctx context.Context, value any) error
}

// Built-in hooks:
- LogLevelHook{}
- HTTPTimeoutHook{}
- LLMTimeoutHook{}
- OTelSampleRatioHook{}
- FeatureFlagHook{prefix: "feature_flags."}
```

## Listener

```go
listener.Listen("cache_invalidate_runtime_configs", func(payload string) {
  ev := parsePayload(payload)
  cfg := loadFromDB(ev.Key)
  hook := registry.Get(ev.Key)
  if err := hook.Apply(ctx, cfg.Value); err != nil {
    slog.Error("config apply failed", "key", ev.Key, "err", err)
  }
})
```

## SIGHUP

```go
signal.Notify(sighup, syscall.SIGHUP)
go func() {
  for range sighup {
    reloadAllConfigs(ctx)
  }
}()
```

## TDD plan

1. Cambio en DB → propaga a todos pods
2. SIGHUP reload
3. Validator rechaza inválido
4. Non-reloadable 409
5. Apply fail → log error + revert opcional
6. Audit log
