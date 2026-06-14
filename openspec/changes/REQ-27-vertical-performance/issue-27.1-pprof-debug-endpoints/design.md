# Design: issue-27.1-pprof-debug-endpoints

## Setup

```go
import _ "net/http/pprof"

func StartDebugServer(cfg DebugConfig) {
  if !cfg.Enabled { return }
  mux := http.NewServeMux()
  mux.HandleFunc("/debug/pprof/", basicAuth(pprof.Index, cfg))
  mux.HandleFunc("/debug/pprof/cmdline", basicAuth(pprof.Cmdline, cfg))
  mux.HandleFunc("/debug/pprof/profile", basicAuth(pprof.Profile, cfg))
  // ... etc
  go http.ListenAndServe(cfg.BindAddr+":"+cfg.Port, mux)
}
```

## Env vars

```
DOMAIN_DEBUG_ENABLED=false
DOMAIN_DEBUG_BIND=127.0.0.1
DOMAIN_DEBUG_PORT=6060
DOMAIN_DEBUG_AUTH_USER=sre
DOMAIN_DEBUG_AUTH_PASSWORD=...
```

## TDD plan

1. Disabled → endpoints 404
2. Enabled + auth → 200 con dump
3. Wrong auth → 401
4. Bind 0.0.0.0 explícito → warning + permite (override consciente)
5. Audit log entry
