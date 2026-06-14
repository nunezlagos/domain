# Design: HU-28.5-fix-ignored-errors

## Estrategia por caso

### JSON encode (HTTP response)

```go
// internal/api/handler/api.go
func writeJSON(w http.ResponseWriter, status int, body any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(body); err != nil {
        slog.Error("failed to encode response", "error", err, "status", status)
    }
}
```

Best effort: si el header ya se escribió, no podemos cambiar el status. Pero al menos se loggea.

### Audit errors

```go
// Antes:
_ = s.Audit.Record(ctx, audit.Entry{...})

// Después:
if err := s.Audit.Record(ctx, audit.Entry{...}); err != nil {
    slog.Warn("audit record failed",
        "error", err,
        "action", "session_end",
        "session_id", sessionID)
}
```

No bloquea la operación principal (audit es best-effort por diseño), pero se loggea para alertas.

### LLM marshal errors

```go
// Antes:
raw, _ := json.Marshal(req)

// Después:
raw, err := json.Marshal(req)
if err != nil {
    return nil, fmt.Errorf("marshal request: %w", err)
}
```

Se propaga porque un request mal formado causa errores crípticos del provider.

### MCP rollback

```go
// Antes:
_ = commitOrRollback(ctx, tx, nil)

// Después:
if err := commitOrRollback(ctx, tx, nil); err != nil {
    slog.Error("mcp tx rollback failed", "error", err)
}
```

Best effort: la tx ya falló, el rollback es cleanup. Pero se loggea.
