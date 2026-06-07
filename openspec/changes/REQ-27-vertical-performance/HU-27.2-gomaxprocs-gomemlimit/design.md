# Design: HU-27.2-gomaxprocs-gomemlimit

## Setup

```go
// cmd/domain-mcp/main.go
import _ "go.uber.org/automaxprocs"  // auto-detect cgroup CPU
import "runtime/debug"

func init() {
  if limit, ok := readCgroupMemoryLimit(); ok {
    debug.SetMemoryLimit(int64(float64(limit) * 0.9))
    slog.Info("GOMEMLIMIT set", "bytes", debug.SetMemoryLimit(-1))
  }
  if gogc := os.Getenv("DOMAIN_GOGC"); gogc != "" {
    if v, err := strconv.Atoi(gogc); err == nil {
      debug.SetGCPercent(v)
    }
  }
}

func readCgroupMemoryLimit() (uint64, bool) {
  // cgroup v2
  if b, err := os.ReadFile("/sys/fs/cgroup/memory.max"); err == nil {
    s := strings.TrimSpace(string(b))
    if s == "max" { return 0, false }
    return strconv.ParseUint(s, 10, 64)
  }
  // cgroup v1 fallback
  if b, err := os.ReadFile("/sys/fs/cgroup/memory/memory.limit_in_bytes"); err == nil {
    return strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
  }
  return 0, false
}
```

## /health/runtime

```go
type RuntimeStats struct {
  GOMAXPROCS       int    `json:"gomaxprocs"`
  GOMEMLIMITBytes  int64  `json:"gomemlimit_bytes"`
  NumGoroutine     int    `json:"num_goroutine"`
  AllocBytes       uint64 `json:"alloc_bytes"`
  SysBytes         uint64 `json:"sys_bytes"`
  GCPauseP99Ms     float64 `json:"gc_pause_p99_ms"`
  NumGC            uint32 `json:"num_gc"`
}
```

## TDD plan

1. Container cap 2 CPU → GOMAXPROCS=2
2. Container 2Gi → memlimit ~1.8Gi
3. Env GOMAXPROCS override
4. /health/runtime devuelve valores
5. Test sin cgroup (dev mac) → fallback runtime.NumCPU
