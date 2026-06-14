# Design: issue-26.1-stateless-invariant

## Linter

```
cmd/domain-lint-stateless/main.go
internal/lint/stateless/
  rules.go      # detect global vars mutables, maps sin sync
  whitelist.go  # carga .stateless-allowed.yaml
  ast.go        # walker
```

## Whitelist shape

```yaml
allowed:
  - path: internal/mcp/resilience/cache.go
    var: lruCache
    reason: "LRU con TTL para cache de policies. State no crítico."
  - path: internal/observability/logging/setup.go
    var: defaultLogger
    reason: "Global slog logger, idiomatic Go."
```

## Detection rules

- `var foo SomeType = ...` at package level + mutable methods → flag
- `var foo = make(map[...]...)` global → flag (no sync, possible race)
- `sync.Map` global sin comment `// stateless-ok: ...` → flag

## TDD plan

1. Linter fixture violación → fail
2. Whitelist con reason → pass
3. 2 pods test multi-instance consistency
